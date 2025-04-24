SHELL = /bin/bash
GO := go
CMDS := $(addprefix bin/, $(shell ls ./cmd | grep -v opm))
OPM := $(addprefix bin/, opm)
SPECIFIC_UNIT_TEST := $(if $(TEST),-run $(TEST),)
SPECIFIC_SKIP_UNIT_TEST := $(if $(SKIP),-skip $(SKIP),)
extra_env := $(GOENV)
export PKG := github.com/operator-framework/operator-registry
export GIT_COMMIT := $(or $(SOURCE_GIT_COMMIT),$(shell git rev-parse --short HEAD))
export OPM_VERSION := $(or $(SOURCE_GIT_TAG),$(shell git describe --always --tags HEAD))
export BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

.DEFAULT_GOAL := all

# bingo manages consistent tooling versions for things like kind, kustomize, etc.
include .bingo/Variables.mk

# protoc is not a go binary, so we need a custom recipe for it.
PROTOC := ./tools/bin/protoc
PROTOC_VERSION := 27.0
.PHONY: $(PROTOC)
$(PROTOC):
	./scripts/ensure-protoc.sh $(PROTOC_VERSION)

# define characters
null  :=
space := $(null) #
comma := ,
# default to json1 for sqlite3 and containers_image_openpgp for containers/image
TAGS := json1,containers_image_openpgp

# Cluster to use for e2e testing
CLUSTER ?= ""
ifeq ($(CLUSTER), kind)
# add kind to the list of tags
TAGS := $(TAGS),kind
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
	$(extra_env) $(GO) build $(extra_flags) -tags=$(TAGS) -o $@ ./cmd/$(notdir $@)

.PHONY: $(OPM)
$(OPM): opm_version_flags=-ldflags "-X '$(PKG)/cmd/opm/version.gitCommit=$(GIT_COMMIT)' -X '$(PKG)/cmd/opm/version.opmVersion=$(OPM_VERSION)' -X '$(PKG)/cmd/opm/version.buildDate=$(BUILD_DATE)'"
$(OPM):
	$(extra_env) $(GO) build $(opm_version_flags) $(extra_flags) -tags=$(TAGS) -o $@ ./cmd/$(notdir $@)

.PHONY: build
build: clean $(CMDS) $(OPM)

.PHONY: cross
cross: opm_version_flags=-ldflags "-X '$(PKG)/cmd/opm/version.gitCommit=$(GIT_COMMIT)' -X '$(PKG)/cmd/opm/version.opmVersion=$(OPM_VERSION)' -X '$(PKG)/cmd/opm/version.buildDate=$(BUILD_DATE)'"
cross:
ifeq ($(shell go env GOARCH),amd64)
	GOOS=darwin CC=o64-clang CXX=o64-clang++ CGO_ENABLED=1 $(GO) build $(opm_version_flags) -tags=$(TAGS) -o "bin/darwin-amd64-opm" --ldflags "-extld=o64-clang" ./cmd/opm
	GOOS=windows CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ CGO_ENABLED=1 $(GO) build $(opm_version_flags) -tags=$(TAGS)  -o "bin/windows-amd64-opm" --ldflags "-extld=x86_64-w64-mingw32-gcc" -buildmode=exe ./cmd/opm
endif

.PHONY: static
static: extra_flags=-ldflags '-w -extldflags "-static"' -tags "json1"
static: build

.PHONY: unit
unit:
	$(GO) test -coverprofile=coverage.out --coverpkg=./... $(SPECIFIC_UNIT_TEST) $(SPECIFIC_SKIP_UNIT_TEST) -tags=$(TAGS) $(TEST_RACE) -count=1 ./pkg/... ./alpha/...

.PHONY: tidy
tidy:
	go mod tidy
	go mod verify

.PHONY: verify
verify: tidy codegen lint
	git diff --exit-code

.PHONY: sanity-check
sanity-check:
	# Build a container with the most recent binaries for this project.
	# Does not include the database, which needs to be added separately.
	docker build -f upstream-builder.Dockerfile -t sanity-container .

	# TODO: add more invocations of the opm binary here

	# serve the container for a second, using the bundles.db in testdata
	docker run --rm -it -v "$(shell pwd)"/pkg/lib/indexer/testdata/:/database sanity-container \
		./bin/opm registry serve --database /database/bundles.db --timeout-seconds 1


.PHONY: image-upstream
image-upstream:
	docker build -f upstream-example.Dockerfile .

.PHONY: lint
lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run --build-tags=$(TAGS) $(GOLANGCI_LINT_ARGS)

.PHONY: fix-lint
fix-lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run --fix $(GOLANGCI_LINT_ARGS)

.PHONY: bingo-upgrade
bingo-upgrade: $(BINGO) #EXHELP Upgrade tools
	@for pkg in $$($(BINGO) list | awk '{ print $$3 }' | tail -n +3 | sed 's/@.*//'); do \
		echo -e "Upgrading \033[35m$$pkg\033[0m to latest..."; \
		$(BINGO) get "$$pkg@latest"; \
	done

.PHONY: codegen
codegen: $(PROTOC) $(PROTOC_GEN_GO_GRPC)
	$(PROTOC) --plugin=protoc-gen-go=$(PROTOC_GEN_GO_GRPC) -I pkg/api/ --go_out=pkg/api pkg/api/*.proto
	$(PROTOC) --plugin=protoc-gen-go-grpc=$(PROTOC_GEN_GO_GRPC) -I pkg/api/ --go-grpc_out=pkg/api pkg/api/*.proto

.PHONY: generate-fakes
generate-fakes:
	$(GO) generate ./...

.PHONY: clean
clean:
	@rm -rf ./bin

.PHONY: e2e
e2e: $(GINKGO)
	$(GINKGO) --v --randomize-all --progress --trace --randomize-suites --race $(if $(TEST),-focus '$(TEST)') -tags=$(TAGS) ./test/e2e -- $(if $(SKIPTLS),-skip-tls-verify true) $(if $(USEHTTP),-use-http true)

.PHONY: release
export OPM_IMAGE_REPO ?= quay.io/operator-framework/opm
export IMAGE_TAG ?= $(OPM_VERSION)
export MAJ_MIN_IMAGE_OR_EMPTY := $(call tagged-or-empty,$(shell echo $(OPM_VERSION) | grep -Eo 'v[0-9]+\.[0-9]+'))
export MAJ_IMAGE_OR_EMPTY := $(call tagged-or-empty,$(shell echo $(OPM_VERSION) | grep -Eo 'v[0-9]+'))
# LATEST_TAG is the latest semver tag in HEAD. Used to deduce whether
# OPM_VERSION is the new latest tag, or a prior minor/patch tag, below.
# NOTE: this can only be relied upon if full git history is present.
# An actions/checkout step must use "fetch-depth: 0", for example.
LATEST_TAG := $(shell git tag -l | tr - \~ | sort -V | tr \~ - | tail -n1)
# LATEST_IMAGE_OR_EMPTY is set to OPM_IMAGE_REPO:latest when OPM_VERSION
# is not a prerelase tag and == LATEST_TAG, otherwise the empty string.
# An empty string causes goreleaser to skip building the manifest image for latest,
# which we do not want when cutting a non-latest release (old minor/patch tag).
export LATEST_IMAGE_OR_EMPTY := $(shell \
	echo $(OPM_VERSION) | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$' \
	&& [ "$(shell echo -e "$(OPM_VERSION)\n$(LATEST_TAG)" | sort -rV | head -n1)" == "$(OPM_VERSION)" ] \
	&& echo "$(OPM_IMAGE_REPO):latest" || echo "")
RELEASE_GOOS := $(shell go env GOOS)
RELEASE_ARGS ?= release --clean --snapshot -f release/goreleaser.$(RELEASE_GOOS).yaml

# Note: bingo does not yet support windows (https://github.com/bwplotka/bingo/issues/26)
# so GOOS=windows gets its own way to install goreleaser
ifeq ($(RELEASE_GOOS), windows)
GORELEASER := $(shell pwd)/bin/goreleaser
release: windows-goreleaser-install
else
release: $(GORELEASER)
endif
release:
	$(GORELEASER) $(RELEASE_ARGS)

.PHONY: windows-goreleaser-install
windows-goreleaser-install:
	# manually install goreleaser from the bingo directory in the same way bingo (currently) installs it.
	# This is done to ensure the same version of goreleaser is used across all platforms
	mkdir -p $(dir $(GORELEASER))
	@echo "(re)installing $(GORELEASER)"
	GOWORK=off $(GO) build -mod=mod -modfile=.bingo/goreleaser.mod -o=$(GORELEASER) "github.com/goreleaser/goreleaser"

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

