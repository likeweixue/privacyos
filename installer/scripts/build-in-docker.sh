#!/bin/bash
# ============================================================================
# 在 Debian 13 容器中构建 PrivacyOS ISO（适用于 macOS / Windows / 非 Debian）
#
# 前置：本机已安装 docker 或 podman，且已运行 build-payload.sh
# 用法:
#   ./installer/scripts/build-in-docker.sh \
#       --source "/path/to/debian-13.7.0-amd64-DVD-1.iso"
# ============================================================================
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SOURCE=""
OUTPUT=""
IMAGE="privacyos-iso-builder:13"
ENGINE=""

while [ $# -gt 0 ]; do
    case "$1" in
        --source) SOURCE="$2"; shift 2 ;;
        --output) OUTPUT="$2"; shift 2 ;;
        --image)  IMAGE="$2"; shift 2 ;;
        -h|--help) sed -n '2,12p' "$0"; exit 0 ;;
        *) echo "未知参数: $1" >&2; exit 2 ;;
    esac
done

if command -v docker >/dev/null 2>&1; then ENGINE=docker
elif command -v podman >/dev/null 2>&1; then ENGINE=podman
else echo "未找到 docker 或 podman" >&2; exit 1; fi

[ -n "${SOURCE}" ] || { echo "请用 --source 指定 debian-13.7.0-amd64-DVD-1.iso" >&2; exit 2; }
SOURCE="$(cd "$(dirname "${SOURCE}")" && pwd)/$(basename "${SOURCE}")"
[ -f "${SOURCE}" ] || { echo "找不到源 ISO: ${SOURCE}" >&2; exit 2; }
[ -f "${ROOT}/build/payload/privacyos-server" ] || {
    echo "载荷未构建，请先执行: ./installer/scripts/build-payload.sh" >&2; exit 1
}

VERSION="$(tr -d '[:space:]' < "${ROOT}/VERSION")"
mkdir -p "${ROOT}/build"
OUTPUT="${OUTPUT:-${ROOT}/build/privacyos-${VERSION}-amd64.iso}"
OUTPUT="$(cd "$(dirname "${OUTPUT}")" && pwd)/$(basename "${OUTPUT}")"

echo "[docker] 构建镜像 ${IMAGE} ..."
"${ENGINE}" build -t "${IMAGE}" -f "${ROOT}/installer/Dockerfile.builder" "${ROOT}/installer"

echo "[docker] 开始构建 ISO ..."
"${ENGINE}" run --rm \
    -u "$(id -u):$(id -g)" \
    -e HOME=/tmp \
    -v "${ROOT}:/work" \
    -v "$(dirname "${SOURCE}"):/iso-src:ro" \
    -v "$(dirname "${OUTPUT}"):/out" \
    -w /work \
    "${IMAGE}" \
    --source "/iso-src/$(basename "${SOURCE}")" \
    --payload /work/build/payload \
    --work /work/build/iso-work \
    --output "/out/$(basename "${OUTPUT}")"

echo "[docker] 完成: ${OUTPUT}"
