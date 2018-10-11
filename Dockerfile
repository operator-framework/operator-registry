FROM golang:1.10-alpine

RUN apk update && apk add sqlite build-base
WORKDIR /go/src/github.com/operator-framework/operator-registry

COPY vendor vendor
COPY cmd cmd
COPY pkg pkg
RUN go build --tags json1 -o ./initializer ./cmd/init/...

COPY manifests manifests
RUN ./initializer


