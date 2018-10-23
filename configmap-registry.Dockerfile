FROM golang:1.11-alpine as builder

RUN apk update && apk add sqlite build-base
WORKDIR /go/src/github.com/operator-framework/operator-registry

COPY vendor vendor
COPY cmd cmd
COPY pkg pkg
COPY Makefile Makefile
RUN make static

FROM scratch
COPY --from=builder /go/src/github.com/operator-framework/operator-registry/bin/configmap-server /configmap-server
EXPOSE 50051
ENTRYPOINT ["/configmap-server"]
