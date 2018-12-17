CMDS  := $(addprefix bin/, $(shell go list ./cmd/... | xargs -I{} basename {}))

.PHONY: build test vendor clean

all: clean test build

$(CMDS):
	go build -mod=vendor -tags json1 $(extra_flags) -o $@ ./cmd/$(shell basename $@)

build: clean $(CMDS)

static: extra_flags=-ldflags '-w -extldflags "-static"'
static: build

test:
	go test -mod=vendor --tags json1 -v -race ./pkg/...

image:
	docker build .

vendor:
	go mod vendor

codegen:
	protoc -I pkg/api/ -I${GOPATH}/src --go_out=plugins=grpc:pkg/api pkg/api/*.proto
	protoc -I pkg/api/grpc_health_v1 -I${GOPATH}/src --go_out=plugins=grpc:pkg/api/grpc_health_v1 pkg/api/grpc_health_v1/*.proto

clean:
	@rm -rf ./bin

