GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
CMDS  := $(addprefix bin/$(GOOS)-$(GOARCH)-, $(shell ls ./cmd))
SPECIFIC_UNIT_TEST := $(if $(TEST),-run $(TEST),)
MOD_FLAGS := $(shell bash -c 'if [[ "$(shell go env GOFLAGS)" == "-mod=vendor" ]]; then echo ""; else echo "-mod=vendor"; fi')

.PHONY: build test vendor clean

all: clean test build

$(CMDS):
	go build $(MOD_FLAGS) $(extra_flags) -o $@ ./cmd/$(shell basename $@ | cut -d- -f3-)

build: clean $(CMDS)

static: extra_flags=-ldflags '-w -extldflags "-static"'
static: build

unit:
	go test $(MOD_FLAGS) $(SPECIFIC_UNIT_TEST) -count=1 -v -race ./pkg/...

image:
	docker build .

image-upstream:
	docker build -f upstream-example.Dockerfile .

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

generate-fakes:
	go generate ./...

clean:
	@rm -rf ./bin

opm-test:
	 $(shell ./opm-test.sh || echo "opm-test FAIL")
