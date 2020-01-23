FROM openshift/origin-release:golang-1.13 as builder

RUN yum update -y && \
    yum install -y make git sqlite glibc-static openssl-static zlib-static && \
    yum groupinstall -y "Development Tools" "Development Libraries"

ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

WORKDIR /src

COPY vendor vendor
COPY cmd cmd
COPY pkg pkg
COPY Makefile go.mod go.sum ./
RUN make build

# copy and build vendored grpc_health_probe
RUN mkdir -p /go/src/github.com/grpc-ecosystem && \
    cp -R vendor/github.com/grpc-ecosystem/grpc-health-probe /go/src/github.com/grpc-ecosystem/grpc_health_probe && \
    cp -R vendor/ /go/src/github.com/grpc-ecosystem/grpc_health_probe && \
    rm -rf /go/src/github.com/grpc-ecosystem/grpc_health_probe/vendor/github.com/grpc-ecosystem/grpc-health-probe && \
    cd /go/src/github.com/grpc-ecosystem/grpc_health_probe && \
    CGO_ENABLED=0 go install -a -tags netgo -ldflags "-w"

FROM openshift/origin-base

RUN mkdir /registry
WORKDIR /registry

COPY --from=builder /src/bin/linux-amd64-initializer /bin/initializer
COPY --from=builder /src/bin/linux-amd64-registry-server /bin/registry-server
COPY --from=builder /src/bin/linux-amd64-configmap-server /bin/configmap-server
COPY --from=builder /src/bin/linux-amd64-appregistry-server /bin/appregistry-server
COPY --from=builder /src/bin/linux-amd64-opm /bin/opm
COPY --from=builder /go/bin/grpc-health-probe /bin/grpc_health_probe

RUN chgrp -R 0 /registry && \
    chgrp -R 0 /dev && \
    chmod -R g+rwx /registry && \
    chmod -R g+rwx /dev

# This image doesn't need to run as root user
USER 1001

EXPOSE 50051

LABEL io.k8s.display-name="OpenShift Operator Registry" \
    io.k8s.description="This is a component of OpenShift Operator Lifecycle Manager and is the base for operator catalog API containers." \
    maintainer="Odin Team <aos-odin@redhat.com>" \
    summary="Operator Registry runs in a Kubernetes or OpenShift cluster to provide operator catalog data to Operator Lifecycle Manager."
