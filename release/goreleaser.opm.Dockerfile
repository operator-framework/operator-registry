# NOTE: This Dockerfile is used in conjuction with GoReleaser to
#   build opm images. See the configurations in .goreleaser.yaml
#   and .github/workflows/release.yaml.

FROM --platform=$BUILDPLATFORM ghcr.io/grpc-ecosystem/grpc-health-probe:v0.4.4 as grpc_health_probe
FROM gcr.io/distroless/static:debug
COPY --from=grpc_health_probe /grpc_health_probe /bin/grpc_health_probe
COPY ["nsswitch.conf", "/etc/nsswitch.conf"]
COPY opm /bin/opm
ENTRYPOINT ["/bin/opm"]
