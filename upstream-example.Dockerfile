FROM quay.io/operator-framework/upstream-registry-builder as builder

COPY manifests manifests
RUN /bin/initializer -o ./bundles.db
RUN mkdir /emptydir && chmod 1777 /emptydir

FROM scratch
COPY --from=builder /build/bundles.db /bundles.db
COPY --from=builder /bin/registry-server /registry-server
COPY --from=builder /bin/grpc_health_probe /bin/grpc_health_probe
COPY --from=builder /emptydir /tmp
EXPOSE 50051
ENTRYPOINT ["/registry-server"]
CMD ["--database", "bundles.db"]
