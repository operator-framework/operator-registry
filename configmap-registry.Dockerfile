FROM golang:1.11-alpine as builder

RUN apk update && apk add sqlite build-base git
WORKDIR /build

COPY vendor vendor
COPY cmd cmd
COPY pkg pkg
COPY Makefile Makefile
COPY go.mod go.mod
COPY go.sum go.sum
RUN make static

FROM scratch
COPY --from=builder /build/bin/configmap-server /configmap-server
EXPOSE 50051
ENTRYPOINT ["/configmap-server"]
