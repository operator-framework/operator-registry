FROM golang:1.10-alpine as builder

RUN apk update && apk add sqlite build-base
WORKDIR /go/src/github.com/operator-framework/operator-registry

COPY vendor vendor
COPY cmd cmd
COPY pkg pkg
COPY Makefile Makefile
RUN make static

COPY manifests manifests
RUN ./bin/initializer -o ./bundles.db

FROM scratch
COPY --from=builder /go/src/github.com/operator-framework/operator-registry/bundles.db /bundles.db
COPY --from=builder /go/src/github.com/operator-framework/operator-registry/bin/registry-server /registry-server
EXPOSE 50051
ENTRYPOINT ["/registry-server"]
CMD ["--database", "bundles.db"]
