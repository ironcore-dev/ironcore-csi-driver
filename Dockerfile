# syntax=docker/dockerfile:experimental
# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:1.17.8 as builder
ARG DEBIAN_FRONTEND=noninteractive
ARG GOARCH=''
ARG GITHUB_PAT=''

WORKDIR /
ADD . .

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

COPY hack hack

ENV GOPRIVATE='github.com/onmetal/*'

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=ssh --mount=type=secret,id=github_pat \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    GITHUB_PAT_PATH=/run/secrets/github_pat ./hack/setup-git-redirect.sh \
    && mkdir -p -m 0600 ~/.ssh \
    && ssh-keyscan github.com >> ~/.ssh/known_hosts \
    && go mod download

# Copy the go source
COPY cmd/ cmd/
COPY pkg/ pkg/

ARG TARGETOS
ARG TARGETARCH

# Build
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GO111MODULE=on go build -ldflags="-s -w" -a -o onmetal-csi-driver cmd/onmetalcsi.go

FROM ubuntu:18.04

COPY --from=builder /onmetal-csi-driver /onmetal-csi-driver
COPY /scripts/env.sh /env.sh
RUN chmod +x /env.sh
COPY onmetal-csi-driver /onmetal-csi-driver

RUN mkdir /onmetal
ADD /scripts/chroot.sh /onmetal
RUN chmod 777 /onmetal/chroot.sh
RUN    ln -s /onmetal/host-chroot.sh /onmetal/blkid \
    && ln -s /onmetal/host-chroot.sh /onmetal/blockdev \
    && ln -s /onmetal/host-chroot.sh /onmetal/iscsiadm \
    && ln -s /onmetal/host-chroot.sh /onmetal/rpcbind \
    && ln -s /onmetal/host-chroot.sh /onmetal/lsblk \
    && ln -s /onmetal/host-chroot.sh /onmetal/lsscsi \
    && ln -s /onmetal/host-chroot.sh /onmetal/mkfs.ext3 \
    && ln -s /onmetal/host-chroot.sh /onmetal/mkfs.ext4 \
    && ln -s /onmetal/host-chroot.sh /onmetal/mkfs.xfs \
    && ln -s /onmetal/host-chroot.sh /onmetal/fsck \
    && ln -s /onmetal/host-chroot.sh /onmetal/mount \
    && ln -s /onmetal/host-chroot.sh /onmetal/multipath \
    && ln -s /onmetal/host-chroot.sh /onmetal/multipathd \
    && ln -s /onmetal/host-chroot.sh /onmetal/cat \
    && ln -s /onmetal/host-chroot.sh /onmetal/mkdir \
    && ln -s /onmetal/host-chroot.sh /onmetal/rmdir \
    && ln -s /onmetal/host-chroot.sh /onmetal/umount

ENV PATH="/onmetal:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"


ENTRYPOINT ["/env.sh"]