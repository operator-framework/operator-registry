FROM openshift/origin-release:golang-1.14 as builder

RUN yum update -y && \
    yum install -y make git sqlite glibc-static openssl-static zlib-static && \
    yum groupinstall -y "Development Tools" "Development Libraries"

ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

WORKDIR /src

COPY OPM_VERSION OPM_VERSION
COPY vendor vendor
COPY cmd cmd
COPY pkg pkg
COPY Makefile go.mod go.sum ./
RUN make build

# copy and build vendored grpc_health_probe
RUN CGO_ENABLED=0 go build -mod=vendor -tags netgo -ldflags "-w" ./vendor/github.com/grpc-ecosystem/grpc-health-probe/...

FROM openshift/origin-base

RUN mkdir /registry
WORKDIR /registry

COPY --from=builder /src/bin/initializer /bin/initializer
COPY --from=builder /src/bin/registry-server /bin/registry-server
COPY --from=builder /src/bin/configmap-server /bin/configmap-server
COPY --from=builder /src/bin/appregistry-server /bin/appregistry-server
COPY --from=builder /src/bin/opm /bin/opm
COPY --from=builder /src/grpc-health-probe /bin/grpc_health_probe

RUN chgrp -R 0 /registry && \
    chmod -R g+rwx /registry

# This image doesn't need to run as root user
USER 1001

EXPOSE 50051

ENTRYPOINT ["/bin/registry-server"]
CMD ["--database", "/bundles.db"]

LABEL io.k8s.display-name="OpenShift Operator Registry" \
    io.k8s.description="This is a component of OpenShift Operator Lifecycle Manager and is the base for operator catalog API containers." \
    maintainer="Odin Team <aos-odin@redhat.com>" \
    summary="Operator Registry runs in a Kubernetes or OpenShift cluster to provide operator catalog data to Operator Lifecycle Manager."
