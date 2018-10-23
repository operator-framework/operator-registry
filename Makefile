CMDS  := $(addprefix bin/, $(shell go list ./cmd/... | xargs -I{} basename {}))

.PHONY: build test vendor clean

all: clean test build

$(CMDS):
	go build -tags json1 $(extra_flags) -o $@ ./cmd/$(shell basename $@)

build: clean $(CMDS)

static: extra_flags=-ldflags '-w -extldflags "-static"'
static: build

test:
	go test --tags json1 -v -race ./pkg/...

image:
	docker build .

vendor:
	dep ensure -v

codegen:
	protoc -I pkg/api/ -I${GOPATH}/src --go_out=plugins=grpc:pkg/api pkg/api/registry.proto

clean:
	rm -rf ./bin
