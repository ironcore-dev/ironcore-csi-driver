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
 
FROM k8s.gcr.io/build-image/debian-base:buster-v1.9.0 as debian
RUN clean-install util-linux e2fsprogs mount ca-certificates udev xfsprogs bash 

FROM gcr.io/distroless/base-debian11
# Copy necessary dependencies into distroless base.
COPY --from=builder /onmetal-csi-driver .
COPY --from=debian /etc/mke2fs.conf /etc/mke2fs.conf
COPY --from=debian /lib/udev/scsi_id /lib/udev_containerized/scsi_id
COPY --from=debian /bin/mount /bin/mount
COPY --from=debian /bin/umount /bin/umount
COPY --from=debian /sbin/blkid /sbin/blkid
COPY --from=debian /sbin/blockdev /sbin/blockdev
COPY --from=debian /sbin/dumpe2fs /sbin/dumpe2fs
COPY --from=debian /sbin/e* /sbin/
COPY --from=debian /sbin/e2fsck /sbin/e2fsck
COPY --from=debian /sbin/fsck /sbin/fsck
COPY --from=debian /sbin/fsck* /sbin/
COPY --from=debian /sbin/fsck.xfs /sbin/fsck.xfs
COPY --from=debian /sbin/mke2fs /sbin/mke2fs
COPY --from=debian /sbin/mkfs* /sbin/
COPY --from=debian /sbin/resize2fs /sbin/resize2fs
COPY --from=debian /sbin/xfs_repair /sbin/xfs_repair
COPY --from=debian /usr/include/xfs /usr/include/xfs
COPY --from=debian /usr/lib/xfsprogs/xfs* /usr/lib/xfsprogs/
COPY --from=debian /usr/sbin/xfs* /usr/sbin/
# Add dependencies for /lib/udev_containerized/google_nvme_id script

COPY --from=debian /bin/bash /bin/bash
COPY --from=debian /bin/date /bin/date
COPY --from=debian /bin/grep /bin/grep
COPY --from=debian /bin/sed /bin/sed
COPY --from=debian /bin/ln /bin/ln
 

# # Copy x86 shared libraries into distroless base.
# COPY --from=debian /lib/x86_64-linux-gnu/libblkid.so.1 /lib/x86_64-linux-gnu/libblkid.so.1
# COPY --from=debian /lib/x86_64-linux-gnu/libcom_err.so.2 /lib/x86_64-linux-gnu/libcom_err.so.2
# COPY --from=debian /lib/x86_64-linux-gnu/libext2fs.so.2 /lib/x86_64-linux-gnu/libext2fs.so.2
# COPY --from=debian /lib/x86_64-linux-gnu/libe2p.so.2 /lib/x86_64-linux-gnu/libe2p.so.2
# COPY --from=debian /lib/x86_64-linux-gnu/libmount.so.1 /lib/x86_64-linux-gnu/libmount.so.1
# COPY --from=debian /lib/x86_64-linux-gnu/libpcre.so.3 /lib/x86_64-linux-gnu/libpcre.so.3
# COPY --from=debian /lib/x86_64-linux-gnu/libreadline.so.5 /lib/x86_64-linux-gnu/libreadline.so.5
# COPY --from=debian /lib/x86_64-linux-gnu/libselinux.so.1 /lib/x86_64-linux-gnu/libselinux.so.1
# COPY --from=debian /lib/x86_64-linux-gnu/libtinfo.so.6 /lib/x86_64-linux-gnu/libtinfo.so.6
# COPY --from=debian /lib/x86_64-linux-gnu/libuuid.so.1 /lib/x86_64-linux-gnu/libuuid.so.1

USER root  
ENTRYPOINT ["/onmetal-csi-driver"]


