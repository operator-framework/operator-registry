# NOTE: This Dockerfile is used in conjuction with GoReleaser to
#   build opm images. See the configurations in .goreleaser.yaml
#   and .github/workflows/release.yaml.

FROM ghcr.io/grpc-ecosystem/grpc-health-probe:v0.4.11 as grpc_health_probe
FROM gcr.io/distroless/static:debug
COPY --from=grpc_health_probe /ko-app/grpc-health-probe /bin/grpc_health_probe
COPY ["nsswitch.conf", "/etc/nsswitch.conf"]
COPY opm /bin/opm
USER 1001
ENTRYPOINT ["/bin/opm"]
