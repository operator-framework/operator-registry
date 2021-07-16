# NOTE: This Dockerfile is used in conjuction with GoReleaser to
#   build opm images. See the configurations in .goreleaser.yaml
#   and .github/workflows/release.yaml.

FROM gcr.io/distroless/base:debug
COPY ["nsswitch.conf", "/etc/nsswitch.conf"]
COPY opm /bin/opm
ENTRYPOINT ["/bin/opm"]
