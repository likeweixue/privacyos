# PrivacyOS ISO builder (Debian 13)
# 只负责重新打包 ISO；后端/前端由宿主机 build-payload.sh 构建。
FROM debian:13-slim

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
        xorriso isolinux syslinux-common syslinux-utils mtools dosfstools \
        ca-certificates coreutils findutils file \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /work
ENTRYPOINT ["/work/installer/scripts/build-iso.sh"]
