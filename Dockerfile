FROM openshift/origin-release:golang-1.14 as builder

ENV OSX_CROSS_PATH=/osxcross

COPY MacOSX10.10.sdk.tar.xz "${OSX_CROSS_PATH}/tarballs/MacOSX10.10.tar.xz"

RUN yum update -y && \
    yum install -y make git sqlite llvm llvm-libs clang libtool cmake3 gcc-c++ dnf && \
    dnf install -y dnf-plugins-core && \
    dnf copr enable -y alonid/mingw-epel7 && \
    dnf install -y mingw64-gcc && \
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
