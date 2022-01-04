# NOTE: This Dockerfile is used in conjuction with GoReleaser to
#   build opm images. See the configurations in .goreleaser.yaml
#   and .github/workflows/release.yaml.

FROM alpine as grpc_health_probe
ARG TARGETARCH
RUN apk update && \
	apk add curl && \
	curl --silent --show-error --location --output /grpc_health_probe \
	https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.5/grpc_health_probe-linux-$TARGETARCH && \
	chmod 755 /grpc_health_probe
FROM gcr.io/distroless/static:debug
COPY --from=grpc_health_probe /grpc_health_probe /bin/grpc_health_probe
COPY ["nsswitch.conf", "/etc/nsswitch.conf"]
COPY opm /bin/opm
ENTRYPOINT ["/bin/opm"]
