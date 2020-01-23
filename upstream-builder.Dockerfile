FROM golang:1.13-alpine

RUN apk update && apk add sqlite build-base git mercurial bash
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
RUN cp /build/bin/linux-amd64-opm /bin/opm && \
    cp /build/bin/linux-amd64-initializer /bin/initializer && \
    cp /build/bin/linux-amd64-appregistry-server /bin/appregistry-server && \
    cp /build/bin/linux-amd64-configmap-server /bin/configmap-server && \
    cp /build/bin/linux-amd64-registry-server /bin/registry-server
