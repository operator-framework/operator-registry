CMDS  := $(addprefix bin/, $(shell go list ./cmd/... | xargs -I{} basename {}))
MOD_FLAGS := $(shell (go version | grep -q 1.11) && echo -mod=vendor)

.PHONY: build test vendor clean

all: clean test build

$(CMDS):
	CGO_ENABLE=0 go build $(MOD_FLAGS) -tags json1 $(extra_flags) -o $@ ./cmd/$(shell basename $@)

build: clean $(CMDS)

static: envs=CGO_ENABLE=0
static: extra_flags=-ldflags '-w -extldflags "-static"' 
static: build

test:
	go test $(MOD_FLAGS) --tags json1 -v -race ./pkg/...

image:
	docker build .

image-upstream:
	docker build -f upstream.Dockerfile .

vendor:
	go mod vendor

codegen:
	protoc -I pkg/api/ -I${GOPATH}/src --go_out=plugins=grpc:pkg/api pkg/api/*.proto
	protoc -I pkg/api/grpc_health_v1 -I${GOPATH}/src --go_out=plugins=grpc:pkg/api/grpc_health_v1 pkg/api/grpc_health_v1/*.proto

clean:
	@rm -rf ./bin

