FROM golang:1.16-alpine

RUN apk update && \
    apk add make git protobuf

ENV MODULE google.golang.org
ENV SRC ${GOPATH}/src/${MODULE}
COPY vendor/${MODULE} ${SRC}
RUN echo $(ls ${SRC})
RUN go install ${SRC}/protobuf/proto ${SRC}/protobuf/cmd/protoc-gen-go ${SRC}/grpc/cmd/protoc-gen-go-grpc


WORKDIR /codegen

COPY pkg pkg
COPY Makefile Makefile
RUN make codegen

LABEL maintainer="Odin Team <aos-odin@redhat.com>"
