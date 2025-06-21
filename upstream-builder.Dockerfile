FROM golang:1.24-alpine AS builder

RUN apk update && apk add sqlite build-base git mercurial bash linux-headers
WORKDIR /build

COPY . .
RUN make static
RUN GRPC_HEALTH_PROBE_VERSION=$(go list -m github.com/grpc-ecosystem/grpc-health-probe| awk '{print $2}') && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-$(go env GOARCH) && \
    chmod +x /bin/grpc_health_probe

FROM alpine:3

COPY ["nsswitch.conf", "/etc/nsswitch.conf"]

COPY --from=builder [ \
    "/bin/grpc_health_probe", \
    "/build/bin/opm", \
    "/build/bin/initializer", \
    "/build/bin/configmap-server", \
    "/build/bin/registry-server", \
    "/bin/" \
]
