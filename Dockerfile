FROM ubuntu:20.04

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