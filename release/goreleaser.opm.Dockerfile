# NOTE: This Dockerfile is used in conjuction with GoReleaser to
#   build opm images. See the configurations in .goreleaser.yaml
#   and .github/workflows/release.yaml.
#
# The GRPC_HEALTH_PROBE_VERSION is automatically passed as a build arg
# by GoReleaser from the GRPC_HEALTH_PROBE_VERSION environment variable,
# which is set in the Makefile from go.mod.

ARG GRPC_HEALTH_PROBE_VERSION
FROM ghcr.io/grpc-ecosystem/grpc-health-probe:${GRPC_HEALTH_PROBE_VERSION} AS grpc_health_probe
FROM gcr.io/distroless/static:debug
COPY --from=grpc_health_probe /ko-app/grpc-health-probe /bin/grpc_health_probe
COPY ["nsswitch.conf", "/etc/nsswitch.conf"]
COPY opm /bin/opm
USER 1001
ENTRYPOINT ["/bin/opm"]
