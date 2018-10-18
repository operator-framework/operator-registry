.PHONY: build test vendor

all: test build

test:
	go test --tags json1 -v -race ./pkg/...

build:
	go build --tags json1 -o ./bin/initializer ./cmd/init/...

image:
	docker build .

vendor:
	dep ensure -v
