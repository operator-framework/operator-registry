FROM quay.io/operator-framework/upstream-registry-builder:latest as builder
FROM busybox as userspace

FROM scratch
COPY --from=builder /bin/configmap-server /bin/configmap-server
COPY --from=builder /bin/opm /bin/opm
COPY --from=userspace /bin/cp /bin/cp
COPY --from=builder /bin/grpc_health_probe /bin/grpc_health_probe
EXPOSE 50051
USER 1001
ENTRYPOINT ["/bin/configmap-server"]
