FROM quay.io/operator-framework/upstream-registry-builder:latest as builder
FROM busybox as userspace

FROM scratch
COPY ["nsswitch.conf", "/etc/nsswitch.conf"]
COPY --from=builder /bin/configmap-server /bin/configmap-server
COPY --from=builder /bin/opm /bin/opm
COPY --from=userspace /bin/cp /bin/cp
COPY --from=builder /bin/grpc_health_probe /bin/grpc_health_probe
COPY --from=userspace --chown=1001:1001 ["/tmp", "/work"]
EXPOSE 50051
USER 1001
WORKDIR /work
ENTRYPOINT ["/bin/configmap-server"]
