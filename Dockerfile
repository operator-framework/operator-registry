FROM openshift/origin-release:golang-1.14 as builder

ENV OSX_CROSS_PATH=/osxcross

COPY MacOSX10.10.sdk.tar.xz "${OSX_CROSS_PATH}/tarballs/MacOSX10.10.tar.xz"

# This is for centos, ignore for rhel
#RUN yum install -y devtoolset-8
#RUN yum update -y && \
#    yum-config-manager --enable epel && \
#    yum install -y make git sqlite llvm llvm-libs clang libtool cmake3 gcc-c++ dnf && \
#    dnf install -y dnf-plugins-core && \
#    dnf copr enable -y alonid/mingw-epel7 && \
#    dnf install -y mingw64-gcc && \
#    ln -s /usr/bin/cmake3 /usr/bin/cmake


# simulate scl enable llvm-toolset-7.0
ENV X_SCLS="${X_SCLS} llvm-toolset-7.0" \
    LD_LIBRARY_PATH=/opt/rh/llvm-toolset-7.0/root/usr/lib64:${LD_LIBRARY_PATH} \
    PATH=/opt/rh/llvm-toolset-7.0/root/usr/bin:/opt/rh/llvm-toolset-7.0/root/usr/sbin:${PATH} \
    PKG_CONFIG_PATH=/opt/rh/llvm-toolset-7.0/root/usr/lib64/pkgconfig:${PKG_CONFIG_PATH}

RUN yum install scl-utils -y
RUN yum-config-manager --disable \* && \
    yum-config-manager --add-repo http://download.eng.bos.redhat.com/brewroot/repos/rhaos-4.5-rhel-7-build/latest/x86_64/ && \
    yum-config-manager --add-repo http://download.eng.bos.redhat.com/brewroot/repos/devtoolset-8.1-rhel-7-container-build/latest/x86_64/
RUN yum update --nogpgcheck -y && \
    yum install --nogpgcheck -y make git sqlite llvm-toolset-7.0 libtool cmake3 gcc-c++ && \
    ln -s /usr/bin/cmake3 /usr/bin/cmake

ARG OSX_CROSS_COMMIT=a9317c18a3a457ca0a657f08cc4d0d43c6cf8953
WORKDIR "${OSX_CROSS_PATH}"
RUN git clone https://github.com/tpoechtrager/osxcross.git \
 && pushd osxcross && git checkout -q "${OSX_CROSS_COMMIT}" \
 && popd && rm -rf ./.git
ARG OSX_VERSION_MIN
COPY MacOSX10.10.sdk.tar.xz osxcross/tarballs/MacOSX10.10.tar.xz
RUN pushd osxcross &&  UNATTENDED=yes OSX_VERSION_MIN=${OSX_VERSION_MIN} ./build.sh && popd

ENV PATH /osxcross/osxcross/target/bin:$GOPATH/bin:/usr/local/go/bin:$PATH

ENV GOPATH /go
WORKDIR /src

COPY vendor vendor
COPY cmd cmd
COPY pkg pkg
COPY Makefile go.mod go.sum ./

ENV CGO_ENABLED=1

# MacOS
RUN GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 CC=o64-clang go install std
ENV GOOS=darwin
ENV GOARCH=amd64
ENV CC=o64-clang
ENV CXX=o64-clang++
ENV LDFLAGS="$LDFLAGS -linkmode external -s"
ENV LDFLAGS_STATIC='-extld='${CC}
ENV TARGET="$GOOS-$GOARCH-opm"
RUN go build -mod=vendor -o "${TARGET}" --ldflags "${LDFLAGS_STATIC}" ./cmd/opm

# Rhel7
RUN yum localinstall -y \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-7/packages/mingw-gcc/4.9.2/3.el7ev/x86_64/mingw64-cpp-4.9.2-3.el7ev.x86_64.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-7/packages/mingw-gcc/4.9.2/3.el7ev/x86_64/mingw64-gcc-4.9.2-3.el7ev.x86_64.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-7/packages/mingw-gcc/4.9.2/3.el7ev/x86_64/mingw64-gcc-c++-4.9.2-3.el7ev.x86_64.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-7/packages/mingw-headers/4.0.2/3.el7ev/noarch/mingw64-headers-4.0.2-3.el7ev.noarch.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-7/packages/mingw-binutils/2.25/1.el7ev/x86_64/mingw-binutils-generic-2.25-1.el7ev.x86_64.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-7/packages/mingw-binutils/2.25/1.el7ev/x86_64/mingw64-binutils-2.25-1.el7ev.x86_64.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-7/packages/mingw-filesystem/100/1.el7ev/noarch/mingw-filesystem-base-100-1.el7ev.noarch.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-7/packages/mingw-filesystem/100/1.el7ev/noarch/mingw64-filesystem-100-1.el7ev.noarch.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-7/packages/mingw-winpthreads/4.0.2/1.el7ev/noarch/mingw64-winpthreads-4.0.2-1.el7ev.noarch.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-7/packages/mingw-winpthreads/4.0.2/1.el7ev/noarch/mingw64-winpthreads-static-4.0.2-1.el7ev.noarch.rpm \
    http://download.eng.bos.redhat.com/brewroot/vol/rhel-7/packages/mingw-crt/4.0.2/1.el7ev/noarch/mingw64-crt-4.0.2-1.el7ev.noarch.rpm

# This should work for rhel8
#RUN yum localinstall -y \
#    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-gcc/7.2.0/2.el8+7/x86_64/mingw64-gcc-c++-7.2.0-2.el8+7.x86_64.rpm \
#    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-gcc/7.2.0/2.el8+7/x86_64/mingw64-cpp-7.2.0-2.el8+7.x86_64.rpm \
#    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-gcc/7.2.0/2.el8+7/x86_64/mingw64-gcc-7.2.0-2.el8+7.x86_64.rpm \
#    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-headers/5.0.2/2.el8_nopadjcc/noarch/mingw64-headers-5.0.2-2.el8_nopadjcc.noarch.rpm \
#    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-crt/5.0.2/2.el8_nopadjcc/noarch/mingw64-crt-5.0.2-2.el8_nopadjcc.noarch.rpm \
#    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-filesystem/104/1.el8_testjcc/noarch/mingw-filesystem-base-104-1.el8_testjcc.noarch.rpm \
#    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-filesystem/104/1.el8_testjcc/noarch/mingw64-filesystem-104-1.el8_testjcc.noarch.rpm \
#    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-binutils/2.30/1.el8_nopadjcc/x86_64/mingw64-binutils-2.30-1.el8_nopadjcc.x86_64.rpm \
#    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-binutils/2.30/1.el8_nopadjcc/x86_64/mingw-binutils-generic-2.30-1.el8_nopadjcc.x86_64.rpm \
#    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-winpthreads/5.0.2/2.el8_nopadjcc/noarch/mingw64-winpthreads-5.0.2-2.el8_nopadjcc.noarch.rpm \
#    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/mingw-winpthreads/5.0.2/2.el8_nopadjcc/noarch/mingw64-winpthreads-static-5.0.2-2.el8_nopadjcc.noarch.rpm \
#    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/glibc/2.28/123.el8/x86_64/glibc-2.28-123.el8.x86_64.rpm \
#    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/glibc/2.28/123.el8/x86_64/glibc-common-2.28-123.el8.x86_64.rpm \
#    http://download.eng.bos.redhat.com/brewroot/vol/rhel-8/packages/glibc/2.28/123.el8/x86_64/glibc-devel-2.28-123.el8.x86_64.rpm

# Windows
RUN GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go install std
ENV GOOS=windows
ENV GOARCH=amd64
ENV CC=x86_64-w64-mingw32-gcc
ENV CXX=x86_64-w64-mingw32-g++
ENV LDFLAGS="$LDFLAGS -linkmode external -s"
ENV LDFLAGS_STATIC='-extld='${CC}
ENV TARGET="$GOOS-$GOARCH-opm"
RUN go build -mod=vendor -o "${TARGET}" --ldflags "${LDFLAGS_STATIC}" ./cmd/opm


FROM scratch
COPY --from=builder /src/windows-amd64-opm /windows-amd64-opm
COPY --from=builder /src/darwin-amd64-opm /darwin-amd64-opm
