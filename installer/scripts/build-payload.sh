#!/bin/bash
# ============================================================================
# 构建 PrivacyOS ISO 载荷（后端二进制 + 前端静态文件 + 安装资源）
#
# 用法:
#   ./installer/scripts/build-payload.sh [--skip-backend] [--skip-frontend]
#
# 环境变量:
#   PRIVACYOS_ADMIN_PASSWORD  指定初始管理员密码（默认随机生成）
#   GO / NPM                 指定工具链路径
# ============================================================================
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BUILD_DIR="${ROOT}/build"
PAYLOAD_DIR="${BUILD_DIR}/payload"
VERSION="$(tr -d '[:space:]' < "${ROOT}/VERSION")"
GO="${GO:-go}"
NPM="${NPM:-npm}"

SKIP_BACKEND=0
SKIP_FRONTEND=0
for arg in "$@"; do
    case "${arg}" in
        --skip-backend) SKIP_BACKEND=1 ;;
        --skip-frontend) SKIP_FRONTEND=1 ;;
        *) echo "未知参数: ${arg}" >&2; exit 2 ;;
    esac
done

log() { printf '\033[1;36m[payload]\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m[payload]\033[0m %s\n' "$*" >&2; exit 1; }

mkdir -p "${BUILD_DIR}"

# ---------------------------------------------------------------------------
# 1. 后端：交叉编译为 linux/amd64 静态二进制
# ---------------------------------------------------------------------------
if [ "${SKIP_BACKEND}" -eq 0 ]; then
    log "编译后端 (linux/amd64)…"
    command -v "${GO}" >/dev/null 2>&1 || die "未找到 go，请安装 Go 1.24+ 或通过 GO=... 指定"
    (
        cd "${ROOT}/backend"
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 "${GO}" build \
            -trimpath \
            -ldflags "-s -w -X privacyos/backend/internal/version.Version=${VERSION}" \
            -o "${BUILD_DIR}/privacyos-server" \
            ./cmd/privacyos-server
    )
    log "后端构建完成: ${BUILD_DIR}/privacyos-server"
else
    [ -f "${BUILD_DIR}/privacyos-server" ] || die "--skip-backend 但缺少 build/privacyos-server"
fi

# ---------------------------------------------------------------------------
# 2. 前端
# ---------------------------------------------------------------------------
if [ "${SKIP_FRONTEND}" -eq 0 ]; then
    log "构建前端…"
    command -v "${NPM}" >/dev/null 2>&1 || die "未找到 npm，请安装 Node.js 20+ 或通过 NPM=... 指定"
    (
        cd "${ROOT}/frontend"
        if [ ! -d node_modules ]; then
            "${NPM}" install --no-audit --no-fund
        fi
        "${NPM}" run build
    )
    [ -f "${ROOT}/frontend/dist/index.html" ] || die "前端构建产物缺失"
else
    [ -f "${ROOT}/frontend/dist/index.html" ] || die "--skip-frontend 但缺少 frontend/dist/index.html"
fi

# ---------------------------------------------------------------------------
# 3. 组装载荷目录
# ---------------------------------------------------------------------------
log "组装安装载荷…"
rm -rf "${PAYLOAD_DIR}"
mkdir -p "${PAYLOAD_DIR}/web" "${PAYLOAD_DIR}/bin"
cp -a "${ROOT}/installer/payload/." "${PAYLOAD_DIR}/"
cp -a "${ROOT}/frontend/dist/." "${PAYLOAD_DIR}/web/"
install -m 0755 "${BUILD_DIR}/privacyos-server" "${PAYLOAD_DIR}/privacyos-server"
printf '%s\n' "${VERSION}" > "${PAYLOAD_DIR}/version"

# 可执行权限（cp -a 会保留源权限，这里再确保一次）
chmod 0755 "${PAYLOAD_DIR}/install.sh" "${PAYLOAD_DIR}/banner.sh" \
    "${PAYLOAD_DIR}/privacyos-provision.sh" "${PAYLOAD_DIR}/privacyos-smart-scan.sh"

# ---------------------------------------------------------------------------
# 4. 初始管理员密码（仅用于首次安装）
# ---------------------------------------------------------------------------
ADMIN_PASSWORD="${PRIVACYOS_ADMIN_PASSWORD:-}"
if [ -z "${ADMIN_PASSWORD}" ]; then
    # Avoid pipefail+SIGPIPE pitfalls: read a bounded block, then filter.
    ADMIN_PASSWORD="$(dd if=/dev/urandom bs=64 count=1 2>/dev/null | base64 | tr -dc 'A-Za-z0-9' | cut -c1-24)"
fi
case "${ADMIN_PASSWORD}" in
    *[!A-Za-z0-9]*)
        die "PRIVACYOS_ADMIN_PASSWORD 只能包含字母和数字，避免 preseed 转义问题" ;;
esac
[ "${#ADMIN_PASSWORD}" -ge 12 ] || die "初始密码至少 12 位"

umask 077
printf '%s\n' "${ADMIN_PASSWORD}" > "${PAYLOAD_DIR}/initial-admin-password"
printf '%s\n' "${ADMIN_PASSWORD}" > "${BUILD_DIR}/initial-admin-password"
umask 022

# ---------------------------------------------------------------------------
# 5. 摘要
# ---------------------------------------------------------------------------
log "载荷目录: ${PAYLOAD_DIR}"
log "初始管理员: padmin / ${ADMIN_PASSWORD}"
log "（凭据同时保存在 build/initial-admin-password，权限 0600）"

cat > "${PAYLOAD_DIR}/README.txt" <<EOF
PrivacyOS ${VERSION} installation payload
=========================================
本目录会被 preseed late_command 复制到目标系统的 /opt/privacyos，
然后执行 install.sh 完成部署。

不要手动修改 web/ 与 privacyos-server，重新构建请运行：
  ./installer/scripts/build-payload.sh
EOF

log "完成"
