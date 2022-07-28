SHELL = /bin/bash
GO := GOFLAGS="-mod=vendor" go
CMDS := $(addprefix bin/, $(shell ls ./cmd | grep -v opm))
OPM := $(addprefix bin/, opm)
SPECIFIC_UNIT_TEST := $(if $(TEST),-run $(TEST),)
extra_env := $(GOENV)
export PKG := github.com/operator-framework/operator-registry
export GIT_COMMIT := $(or $(SOURCE_GIT_COMMIT),$(shell git rev-parse --short HEAD))
export OPM_VERSION := $(or $(SOURCE_GIT_TAG),$(shell git describe --always --tags HEAD))
export BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# define characters
null  :=
space := $(null) #
comma := ,
# default to json1 for sqlite3
TAGS := -tags=json1

# Cluster to use for e2e testing
CLUSTER ?= ""
ifeq ($(CLUSTER), kind)
# add kind to the list of tags
TAGS += kind
# convert tag format from space to comma list
TAGS := $(subst $(space),$(comma),$(strip $(TAGS)))
endif

# -race is only supported on linux/amd64, linux/ppc64le, linux/arm64, freebsd/amd64, netbsd/amd64, darwin/amd64 and windows/amd64
ifeq ($(shell go env GOARCH),s390x)
TEST_RACE :=
else
TEST_RACE := -race
endif

.PHONY: all
all: clean test build

$(CMDS):
	$(extra_env) $(GO) build $(extra_flags) $(TAGS) -o $@ ./cmd/$(notdir $@)

.PHONY: $(OPM)
$(OPM): opm_version_flags=-ldflags "-X '$(PKG)/cmd/opm/version.gitCommit=$(GIT_COMMIT)' -X '$(PKG)/cmd/opm/version.opmVersion=$(OPM_VERSION)' -X '$(PKG)/cmd/opm/version.buildDate=$(BUILD_DATE)'"
$(OPM):
	$(extra_env) $(GO) build $(opm_version_flags) $(extra_flags) $(TAGS) -o $@ ./cmd/$(notdir $@)

.PHONY: build
build: clean $(CMDS) $(OPM)

.PHONY: cross
cross: opm_version_flags=-ldflags "-X '$(PKG)/cmd/opm/version.gitCommit=$(GIT_COMMIT)' -X '$(PKG)/cmd/opm/version.opmVersion=$(OPM_VERSION)' -X '$(PKG)/cmd/opm/version.buildDate=$(BUILD_DATE)'"
cross:
ifeq ($(shell go env GOARCH),amd64)
	GOOS=darwin CC=o64-clang CXX=o64-clang++ CGO_ENABLED=1 $(GO) build $(opm_version_flags) $(TAGS) -o "bin/darwin-amd64-opm" --ldflags "-extld=o64-clang" ./cmd/opm
	GOOS=windows CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ CGO_ENABLED=1 $(GO) build $(opm_version_flags) $(TAGS)  -o "bin/windows-amd64-opm" --ldflags "-extld=x86_64-w64-mingw32-gcc" -buildmode=exe ./cmd/opm
endif

.PHONY: static
static: extra_flags=-ldflags '-w -extldflags "-static"' -tags "json1"
static: build

.PHONY: unit
unit:
	$(GO) test -coverprofile=coverage.out $(SPECIFIC_UNIT_TEST) $(TAGS) $(TEST_RACE) -count=1 ./pkg/... ./alpha/...

.PHONY: sanity-check
sanity-check:
	# Build a container with the most recent binaries for this project.
	# Does not include the database, which needs to be added separately.
	docker build -f upstream-builder.Dockerfile -t sanity-container .

	# TODO: add more invocations of the opm binary here

	# serve the container for a second, using the bundles.db in testdata
	docker run --rm -it -v "$(shell pwd)"/pkg/lib/indexer/testdata/:/database sanity-container \
		./bin/opm registry serve --database /database/bundles.db --timeout-seconds 1

.PHONY: image
image:
	docker build .

.PHONY: image-upstream
image-upstream:
	docker build -f upstream-example.Dockerfile .

.PHONY: vendor
vendor:
	$(GO) mod tidy
	$(GO) mod vendor
	$(GO) mod verify

.PHONY: verify
verify: vendor
	git diff --exit-code

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter checks.
	$(GOLANGCI_LINT) run

.PHONY: codegen
codegen:
	protoc -I pkg/api/ --go_out=pkg/api pkg/api/*.proto
	protoc -I pkg/api/ --go-grpc_out=pkg/api pkg/api/*.proto
	protoc -I pkg/api/grpc_health_v1 --go_out=pkg/api/grpc_health_v1 pkg/api/grpc_health_v1/*.proto
	protoc -I pkg/api/grpc_health_v1 --go-grpc_out=pkg/api/grpc_health_v1 pkg/api/grpc_health_v1/*.proto

.PHONY: container-codegen
container-codegen:
	docker build -t operator-registry:codegen -f codegen.Dockerfile .
	docker run --name temp-codegen operator-registry:codegen /bin/true
	docker cp temp-codegen:/codegen/pkg/api/. ./pkg/api
	docker rm temp-codegen

.PHONY: generate-fakes
generate-fakes:
	$(GO) generate ./...

.PHONY: clean
clean:
	@rm -rf ./bin

.PHONY: e2e
e2e: ginkgo
	$(GINKGO) --v --randomize-all --progress --trace --randomize-suites --race $(if $(TEST),-focus '$(TEST)') $(TAGS) ./test/e2e -- $(if $(SKIPTLS),-skip-tls-verify true) $(if $(USEHTTP),-use-http true)

.PHONY: release
export OPM_IMAGE_REPO ?= quay.io/operator-framework/opm
export IMAGE_TAG ?= $(OPM_VERSION)
export MAJ_MIN_IMAGE_OR_EMPTY ?= $(call tagged-or-empty,$(shell echo $(OPM_VERSION) | grep -Eo 'v[0-9]+\.[0-9]+'))
export MAJ_IMAGE_OR_EMPTY ?= $(call tagged-or-empty,$(shell echo $(OPM_VERSION) | grep -Eo 'v[0-9]+'))
# LATEST_TAG is the latest semver tag in HEAD. Used to deduce whether
# OPM_VERSION is the new latest tag, or a prior minor/patch tag, below.
# NOTE: this can only be relied upon if full git history is present.
# An actions/checkout step must use "fetch-depth: 0", for example.
LATEST_TAG := $(shell git tag -l | tr - \~ | sort -V | tr \~ - | tail -n1)
# LATEST_IMAGE_OR_EMPTY is set to OPM_IMAGE_REPO:latest when OPM_VERSION
# is not a prerelase tag and == LATEST_TAG, otherwise the empty string.
# An empty string causes goreleaser to skip building the manifest image for latest,
# which we do not want when cutting a non-latest release (old minor/patch tag).
export LATEST_IMAGE_OR_EMPTY ?= $(shell \
	echo $(OPM_VERSION) | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$' \
	&& [ "$(shell echo -e "$(OPM_VERSION)\n$(LATEST_TAG)" | sort -rV | head -n1)" == "$(OPM_VERSION)" ] \
	&& echo "$(OPM_IMAGE_REPO):latest" || echo "")
release: RELEASE_ARGS ?= release --rm-dist --snapshot -f release/goreleaser.$(shell go env GOOS).yaml
release: goreleaser
	$(GORELEASER) $(RELEASE_ARGS)

# tagged-or-empty returns $(OPM_IMAGE_REPO):$(1) when HEAD is assigned a non-prerelease semver tag,
# otherwise the empty string. An empty string causes goreleaser to skip building
# the manifest image for a trunk commit when it is not a release commit.
# In other words, this function will return "" if the tag is not in vX.Y.Z format.
define tagged-or-empty
$(shell \
	echo $(OPM_VERSION) | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$' \
	&& git describe --tags --exact-match HEAD >/dev/null 2>&1 \
	&& echo "$(OPM_IMAGE_REPO):$(1)" || echo "" )
endef

################
# Hack / Tools #
################

GO_INSTALL_OPTS ?= "-mod=mod"

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
GORELEASER ?= $(LOCALBIN)/goreleaser
GINKGO ?= $(LOCALBIN)/ginkgo
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint

## Tool Versions
GORELEASER_VERSION ?= v1.8.3
GINKGO_VERSION ?= v2.1.3
GOLANGCI_LINT_VERSION ?= v1.45.2

.PHONY: goreleaser
goreleaser: $(GORELEASER) ## Download goreleaser locally if necessary.
$(GORELEASER): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install $(GO_INSTALL_OPTS) github.com/goreleaser/goreleaser@$(GORELEASER_VERSION)

.PHONY: ginkgo
ginkgo: $(GINKGO) ## Download ginkgo locally if necessary.
$(GINKGO): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install $(GO_INSTALL_OPTS) github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT)
$(GOLANGCI_LINT): $(LOCALBIN) ## Download golangci-lint locally if necessary.
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
