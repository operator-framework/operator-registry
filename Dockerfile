FROM registry.svc.ci.openshift.org/ocp/builder:rhel-8-golang-openshift-4.6 as builder

ENV GOPATH /go
RUN mkdir /src
WORKDIR /src

# Required packages for mac
RUN dnf update -y && \
    dnf install -y patch xz sqlite llvm-toolset libtool cmake3 gcc-c++ libxml2-devel

## Required pacakges for windows
RUN dnf update -y && \
    dnf install -y glibc \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-gcc/7.2.0/2.el8+7/x86_64/mingw64-gcc-c++-7.2.0-2.el8+7.x86_64.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-gcc/7.2.0/2.el8+7/x86_64/mingw64-cpp-7.2.0-2.el8+7.x86_64.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-gcc/7.2.0/2.el8+7/x86_64/mingw64-gcc-7.2.0-2.el8+7.x86_64.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-headers/5.0.2/2.el8_nopadjcc/noarch/mingw64-headers-5.0.2-2.el8_nopadjcc.noarch.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-crt/5.0.2/2.el8_nopadjcc/noarch/mingw64-crt-5.0.2-2.el8_nopadjcc.noarch.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-filesystem/104/1.el8_testjcc/noarch/mingw-filesystem-base-104-1.el8_testjcc.noarch.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-filesystem/104/1.el8_testjcc/noarch/mingw64-filesystem-104-1.el8_testjcc.noarch.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-binutils/2.30/1.el8_nopadjcc/x86_64/mingw64-binutils-2.30-1.el8_nopadjcc.x86_64.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-binutils/2.30/1.el8_nopadjcc/x86_64/mingw-binutils-generic-2.30-1.el8_nopadjcc.x86_64.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-winpthreads/5.0.2/2.el8_nopadjcc/noarch/mingw64-winpthreads-5.0.2-2.el8_nopadjcc.noarch.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-winpthreads/5.0.2/2.el8_nopadjcc/noarch/mingw64-winpthreads-static-5.0.2-2.el8_nopadjcc.noarch.rpm

# Build cross-compilers for mac
ENV OSX_CROSS_PATH /src/osxcross
COPY cross/osxcross ${OSX_CROSS_PATH}
COPY cross/MacOSX10.10.sdk.tar.xz "${OSX_CROSS_PATH}/tarballs/MacOSX10.10.tar.xz"
RUN pushd osxcross && UNATTENDED=yes ./build.sh && popd
ENV PATH $OSX_CROSS_PATH/target/bin:$GOPATH/bin:/usr/local/go/bin:$PATH

COPY vendor vendor
COPY cmd cmd
COPY pkg pkg
COPY Makefile go.mod go.sum ./

# MacOS
RUN GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 CC=o64-clang go install std
RUN GOOS=darwin GOARCH=amd64 CC=o64-clang CXX=o64-clang++ CGO_ENABLED=1 go build -mod=vendor -o "darwin-amd64-opm" --ldflags "-extld=o64-clang" ./cmd/opm

# Windows
RUN GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go install std
RUN GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-c++ CGO_ENABLED=1 go build -mod=vendor -o "windows-amd64-opm" --ldflags "-extld=x86_64-w64-mingw32-gcc" ./cmd/opm

# Standard linux build
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
COPY --from=builder /src/windows-amd64-opm /bin/windows-amd64-opm
COPY --from=builder /src/darwin-amd64-opm /bin/darwin-amd64-opm
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
