FROM golang:1.13-alpine as builder

RUN apk update && apk add sqlite build-base git mercurial bash
WORKDIR /go/src/github.com/operator-framework/operator-registry

COPY vendor vendor
COPY cmd cmd
COPY pkg pkg
COPY Makefile Makefile
COPY go.mod go.mod
RUN make static

FROM golang:1.13-alpine as probe-builder

RUN apk update && apk add build-base git
ENV ORG github.com/grpc-ecosystem
ENV PROJECT $ORG/grpc_health_probe
WORKDIR /go/src/$PROJECT

COPY --from=builder /go/src/github.com/operator-framework/operator-registry/vendor/$ORG/grpc-health-probe .
COPY --from=builder /go/src/github.com/operator-framework/operator-registry/vendor .
RUN CGO_ENABLED=0 go install -a -tags netgo -ldflags "-w"

FROM scratch
COPY --from=builder /go/src/github.com/operator-framework/operator-registry/bin/appregistry-server /bin/appregistry-server
COPY --from=probe-builder /go/bin/grpc-health-probe /bin/grpc_health_probe
EXPOSE 50051
ENTRYPOINT ["/bin/appregistry-server"]
