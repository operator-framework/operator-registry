MOD_FLAGS := $(shell (go version | grep -q -E "1\.(11|12)") && echo -mod=vendor)
CMDS  := $(addprefix bin/, $(shell go list $(MOD_FLAGS) ./cmd/... | xargs -I{} basename {}))

.PHONY: build test vendor clean run-example image-upstream clean-example

all: clean test build

$(CMDS):
	go build $(MOD_FLAGS) $(extra_flags) -o $@ ./cmd/$(shell basename $@)

build: clean $(CMDS)

static: extra_flags=-ldflags '-w -extldflags "-static"'
static: build

unit:
	go test $(MOD_FLAGS) -count=1 -v -race ./pkg/...

image:
	docker build .

run-example: example-image clean-example
	@docker run --name example-registry -p 50051:50051 -d --rm example-registry:latest
	@echo "example registry running and accessible at localhost:50051"

example-image:
	@docker build -f upstream-example.Dockerfile . -t example-registry:latest

clean-example:
	@docker rm -f example-registry || :

vendor:
	go mod vendor

codegen:
	protoc -I pkg/api/ --go_out=plugins=grpc:pkg/api pkg/api/*.proto
	protoc -I pkg/api/grpc_health_v1 --go_out=plugins=grpc:pkg/api/grpc_health_v1 pkg/api/grpc_health_v1/*.proto

container-codegen:
	docker build -t operator-registry:codegen -f codegen.Dockerfile .
	docker run --name temp-codegen operator-registry:codegen /bin/true
	docker cp temp-codegen:/codegen/pkg/api/. ./pkg/api
	docker rm temp-codegen

clean:
	@rm -rf ./bin

