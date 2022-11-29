# syntax=docker/dockerfile:experimental
FROM --platform=$BUILDPLATFORM golang:1.19.3 as builder
ARG DEBIAN_FRONTEND=noninteractive
ARG GOARCH=''

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    go mod download

# Copy the go source
COPY cmd/ cmd/
COPY pkg/ pkg/

ARG TARGETOS
ARG TARGETARCH

# Build
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GO111MODULE=on go build -ldflags="-s -w" -a -o onmetal-csi-driver cmd/onmetalcsi.go
 
FROM k8s.gcr.io/build-image/debian-base:buster-v1.9.0 as debian
COPY --from=builder /workspace/onmetal-csi-driver .
RUN clean-install util-linux e2fsprogs mount ca-certificates udev xfsprogs bash 
  
ENTRYPOINT ["/onmetal-csi-driver"]
