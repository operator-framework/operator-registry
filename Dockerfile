FROM golang:1.11-alpine as builder

RUN apk update && apk add sqlite build-base git mercurial
WORKDIR /build

COPY vendor vendor
COPY cmd cmd
COPY pkg pkg
COPY Makefile Makefile
COPY go.mod go.mod
COPY go.sum go.sum
RUN make static

COPY manifests manifests
RUN ./bin/initializer -o ./bundles.db

FROM golang:1.10-alpine as probe-builder

RUN apk update && apk add build-base git
RUN go get -u github.com/golang/dep/cmd/dep
ENV ORG github.com/grpc-ecosystem
ENV PROJECT $ORG/grpc_health_probe
WORKDIR /go/src/$PROJECT

COPY --from=builder /build/vendor/$ORG/grpc-health-probe .
RUN dep ensure -vendor-only -v && \
    go install -a -tags netgo -ldflags "-linkmode external -extldflags -static"

FROM scratch
COPY --from=builder /build/bundles.db /bundles.db
COPY --from=builder /build/bin/registry-server /registry-server
COPY --from=probe-builder /go/bin/grpc_health_probe /bin/grpc_health_probe
EXPOSE 50051
ENTRYPOINT ["/registry-server"]
CMD ["--database", "bundles.db"]
