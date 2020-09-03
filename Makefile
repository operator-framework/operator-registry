GO := GOFLAGS="-mod=vendor" go
CMDS := $(addprefix bin/, $(shell ls ./cmd | grep -v opm))
OPM := $(addprefix bin/, opm)
SPECIFIC_UNIT_TEST := $(if $(TEST),-run $(TEST),)
PKG := github.com/operator-framework/operator-registry
GIT_COMMIT := $(or $(SOURCE_GIT_COMMIT),$(shell git rev-parse --short HEAD))
OPM_VERSION := $(or $(SOURCE_GIT_TAG),$(shell git describe --always --tags HEAD))
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
TAGS := -tags "json1"
# -race is only supported on linux/amd64, linux/ppc64le, linux/arm64, freebsd/amd64, netbsd/amd64, darwin/amd64 and windows/amd64
ifeq ($(shell go env GOARCH),s390x)
TEST_RACE :=
else
TEST_RACE := -race
endif


.PHONY: all
all: clean test build

$(CMDS):
	$(GO) build $(extra_flags) $(TAGS) -o $@ ./cmd/$(notdir $@)

$(OPM): opm_version_flags=-ldflags "-X '$(PKG)/cmd/opm/version.gitCommit=$(GIT_COMMIT)' -X '$(PKG)/cmd/opm/version.opmVersion=$(OPM_VERSION)' -X '$(PKG)/cmd/opm/version.buildDate=$(BUILD_DATE)'"
$(OPM):
	$(GO) build $(opm_version_flags) $(extra_flags) $(TAGS) -o $@ ./cmd/$(notdir $@)

.PHONY: build
build: clean $(CMDS) $(OPM)

.PHONY: cross
cross: opm_version_flags=-ldflags "-X '$(PKG)/cmd/opm/version.gitCommit=$(GIT_COMMIT)' -X '$(PKG)/cmd/opm/version.opmVersion=$(OPM_VERSION)' -X '$(PKG)/cmd/opm/version.buildDate=$(BUILD_DATE)'"
cross:
ifeq ($(shell go env GOARCH),amd64)
	GOOS=darwin CC=o64-clang CXX=o64-clang++ CGO_ENABLED=1 $(GO) build $(opm_version_flags) $(TAGS) -o "$(PKG)/bin/darwin-amd64-opm" --ldflags "-extld=o64-clang" ./cmd/opm
	GOOS=windows CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ CGO_ENABLED=1 $(GO) build $(opm_version_flags) $(TAGS)  -o "$(PKG)/bin/windows-amd64-opm" --ldflags "-extld=x86_64-w64-mingw32-gcc" ./cmd/opm
endif

.PHONY: static
static: extra_flags=-ldflags '-w -extldflags "-static"' -tags "json1"
static: build

.PHONY: unit
unit:
	$(GO) test $(SPECIFIC_UNIT_TEST) $(TAGS) $(TEST_RACE) -count=1 -v ./pkg/...

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
	$(GO) mod vendor

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
e2e:
	$(GO) run github.com/onsi/ginkgo/ginkgo --v --randomizeAllSpecs --randomizeSuites --race $(TAGS) ./test/e2e
