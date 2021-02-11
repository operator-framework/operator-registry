FROM golang:1.15-alpine AS builder

RUN apk update && apk add sqlite build-base git mercurial bash
WORKDIR /build

COPY . .
RUN make static
RUN GRPC_HEALTH_PROBE_VERSION=v0.3.2 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-$(go env GOARCH) && \
    chmod +x /bin/grpc_health_probe

FROM alpine
RUN apk update && apk add ca-certificates
COPY ["nsswitch.conf", "/etc/nsswitch.conf"]
COPY --from=builder /build/bin/opm /bin/opm
COPY --from=builder /bin/grpc_health_probe /bin/grpc_health_probe
