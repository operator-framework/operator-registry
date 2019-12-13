FROM golang:1.12-alpine AS builder

RUN apk update && apk add sqlite build-base git mercurial
WORKDIR /build

COPY vendor vendor
COPY cmd cmd
COPY pkg pkg
COPY Makefile Makefile
COPY go.mod go.mod
RUN make static
RUN GRPC_HEALTH_PROBE_VERSION=v0.2.1 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

FROM alpine:3.10
COPY --from=builder /build/bin/appregistry-server /bin/appregistry-server
COPY --from=builder /bin/grpc_health_probe /bin/grpc_health_probe

RUN mkdir /registry
RUN chgrp -R 0 /registry && \
    chmod -R g+rwx /registry

WORKDIR /registry

# This image doesn't need to run as root user
USER 1001

EXPOSE 50051
ENTRYPOINT ["/bin/appregistry-server"]
