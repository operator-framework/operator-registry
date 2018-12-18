FROM openshift/origin-release:golang-1.10 as builder

RUN yum update -y && \
    yum install -y make git sqlite && \
    yum groupinstall -y "Development Tools" "Development Libraries"

ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

WORKDIR /go/src/github.com/operator-framework/operator-registry

COPY vendor vendor
COPY cmd cmd
COPY pkg pkg
COPY Makefile Makefile
RUN make static

# copy and build vendored grpc_health_probe
RUN mkdir -p /go/src/github.com/grpc-ecosystem && \
    cp -R vendor/github.com/grpc-ecosystem/grpc-health-probe /go/src/github.com/grpc-ecosystem/grpc_health_probe && \
    cd /go/src/github.com/grpc-ecosystem/grpc_health_probe && \
    go get -u github.com/golang/dep/cmd/dep && \
    dep ensure -vendor-only -v && \
    go install -a -tags netgo -ldflags "-linkmode external   -extldflags -static"

FROM openshift/origin-base

COPY --from=builder /go/src/github.com/operator-framework/operator-registry/bin/initializer /initializer
COPY --from=builder /go/src/github.com/operator-framework/operator-registry/bin/registry-server /registry-server
COPY --from=builder /go/src/github.com/operator-framework/operator-registry/bin/configmap-server /configmap-server
COPY --from=builder /go/bin/grpc_health_probe /bin/grpc_health_probe

# This image doesn't need to run as root user
USER 1001

EXPOSE 50051

LABEL io.k8s.display-name="OpenShift Operator Registry" \
      io.k8s.description="This is a component of OpenShift Operator Lifecycle Manager and is the base for operator catalog API containers." \
      maintainer="Odin Team <aos-odin@redhat.com>"
