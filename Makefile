GO := GOFLAGS="-mod=vendor" go
CMDS := $(addprefix bin/, $(shell ls ./cmd))
SPECIFIC_UNIT_TEST := $(if $(TEST),-run $(TEST),)

.PHONY: all
all: clean test build

$(CMDS):
	$(GO) build $(extra_flags) -o $@ ./cmd/$(notdir $@)

.PHONY: build
build: clean $(CMDS)

.PHONY: static
static: extra_flags=-ldflags '-w -extldflags "-static"'
static: build

.PHONY: unit
unit:
	$(GO) test $(SPECIFIC_UNIT_TEST) -count=1 -v -race ./pkg/...

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
	protoc -I pkg/api/ --go_out=plugins=grpc:pkg/api pkg/api/*.proto
	protoc -I pkg/api/grpc_health_v1 --go_out=plugins=grpc:pkg/api/grpc_health_v1 pkg/api/grpc_health_v1/*.proto

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
	$(GO) run github.com/onsi/ginkgo/ginkgo --v --randomizeAllSpecs --randomizeSuites --race ./test/e2e
