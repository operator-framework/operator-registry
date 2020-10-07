FROM registry.svc.ci.openshift.org/ocp/builder:rhel-8-golang-openshift-4.6 as builder

ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

WORKDIR /src

COPY vendor vendor
COPY cmd cmd
COPY pkg pkg
COPY Makefile go.mod go.sum ./
RUN make build cross

# copy and build vendored grpc_health_probe
RUN CGO_ENABLED=0 go build -mod=vendor -tags netgo -ldflags "-w" ./vendor/github.com/grpc-ecosystem/grpc-health-probe/...

FROM registry.svc.ci.openshift.org/ocp/4.6:base

COPY --from=builder /src/bin/* /tmp/bin/
COPY --from=builder /src/grpc-health-probe /bin/grpc_health_probe

RUN cp -avr /tmp/bin/. /bin/ && rm -rf /tmp/bin

RUN mkdir /registry
RUN chgrp -R 0 /registry && \
    chmod -R g+rwx /registry
WORKDIR /registry

# This image doesn't need to run as root user
USER 1001

EXPOSE 50051

ENTRYPOINT ["/bin/registry-server"]
CMD ["--database", "/bundles.db"]

LABEL io.k8s.display-name="OpenShift Operator Registry" \
      io.k8s.description="This is a component of OpenShift Operator Lifecycle Manager and is the base for operator catalog API containers." \
      maintainer="Odin Team <aos-odin@redhat.com>" \
      summary="Operator Registry runs in a Kubernetes or OpenShift cluster to provide operator catalog data to Operator Lifecycle Manager."
