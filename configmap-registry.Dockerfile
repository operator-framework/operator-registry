FROM quay.io/operator-framework/upstream-registry-builder:latest as builder
FROM busybox as userspace

FROM scratch
COPY --from=builder /build/bin/linux/configmap-server /bin/configmap-server
COPY --from=builder /build/bin/linux/opm /bin/opm
COPY --from=userspace /bin/cp /bin/cp
COPY --from=builder /bin/grpc_health_probe /bin/grpc_health_probe
EXPOSE 50051
ENTRYPOINT ["/bin/configmap-server"]
