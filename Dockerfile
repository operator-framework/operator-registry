FROM golang:1.10-alpine

RUN apk update && apk add sqlite build-base
WORKDIR /go/src/github.com/operator-framework/operator-registry

COPY vendor vendor
COPY init init
RUN go build -o ./initializer ./init/...

COPY manifests manifests
RUN ./initializer


