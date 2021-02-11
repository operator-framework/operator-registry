FROM golang:1.15-alpine as builder

RUN apk update && apk add sqlite build-base git mercurial bash
WORKDIR /build

COPY . .
RUN make static
RUN GRPC_HEALTH_PROBE_VERSION=v0.3.2 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-$(go env GOARCH) && \
    chmod +x /bin/grpc_health_probe

FROM alpine:3

COPY ["nsswitch.conf", "/etc/nsswitch.conf"]

RUN chgrp -R 0 /etc && \
    chmod -R g+rwx /etc

COPY --from=builder [ \
    "/bin/grpc_health_probe", \
    "/build/bin/opm", \
    "/build/bin/initializer", \
    "/build/bin/appregistry-server", \
    "/build/bin/configmap-server", \
    "/build/bin/registry-server", \
    "/bin/" \
]
