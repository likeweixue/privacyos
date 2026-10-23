#!/bin/bash
# ============================================================================
# 构建 PrivacyOS 安装 ISO（基于 debian-13.7.0-amd64-DVD-1.iso）
#
# 必须在 Linux 上运行（需要 xorriso；macOS 无法生成 BIOS+UEFI 混合可引导 ISO）。
# 推荐在 Debian 13 容器中执行：
#   ./installer/scripts/build-in-docker.sh --source /path/debian-13.7.0-amd64-DVD-1.iso
#
# 用法:
#   ./installer/scripts/build-iso.sh [--source ISO] [--payload DIR]
#       [--output ISO] [--work DIR] [--edition normal|pro] [--no-md5] [--keep]
#
# 说明:
#   * 源 ISO 通过 xorriso 解包，无需 root、无需 loop mount
#   * 专业版 LUKS 口令在构建时生成并写入 build/LUKS-PASSPHRASE.txt
#   * 普通版与专业版菜单、preseed 会同时打包；--edition 仅调整默认说明
# ============================================================================
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="$(tr -d '[:space:]' < "${ROOT}/VERSION")"

SOURCE_ISO="${ROOT}/../开发文档/debian-13.7.0-amd64-DVD-1.iso"
PAYLOAD_DIR="${ROOT}/build/payload"
OUTPUT_ISO="${ROOT}/build/privacyos-${VERSION}-amd64.iso"
WORK_DIR="${ROOT}/build/iso-work"
TREE_DIR="${WORK_DIR}/iso"
EDITION="both"
DO_MD5=1
KEEP_WORK=0
NETINST="${PRIVACYOS_NETINST:-0}"
MIN_ISO_SIZE="${PRIVACYOS_MIN_ISO_BYTES:-1000000000}"

log()  { printf '\033[1;36m[iso]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[iso]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m[iso]\033[0m %s\n' "$*" >&2; exit 1; }

while [ $# -gt 0 ]; do
    case "$1" in
        --source)   SOURCE_ISO="$2"; shift 2 ;;
        --payload)  PAYLOAD_DIR="$2"; shift 2 ;;
        --output)   OUTPUT_ISO="$2"; shift 2 ;;
        --work)     WORK_DIR="$2"; TREE_DIR="${WORK_DIR}/iso"; shift 2 ;;
        --edition)  EDITION="$2"; shift 2 ;;
        --no-md5)   DO_MD5=0; shift ;;
        --keep)     KEEP_WORK=1; shift ;;
        -h|--help)  sed -n '2,25p' "$0"; exit 0 ;;
        *) die "未知参数: $1" ;;
    esac
done

case "$(uname -s)" in
    Linux) ;;
    Darwin)
        cat >&2 <<'EOF'
[iso] macOS 无法生成 Debian 的 BIOS+UEFI 混合可引导 ISO（缺少 xorriso 且
      El Torito 工具链行为不同）。请任选一种方式：
        1) 在 Debian 13 / Ubuntu 24.04 中运行本脚本
        2) ./installer/scripts/build-in-docker.sh --source <ISO 路径>
        3) 使用 CI（.github/workflows/ci.yml 的 iso 任务）
EOF
        exit 1 ;;
    *) die "仅支持 Linux 构建环境" ;;
esac

command -v xorriso >/dev/null 2>&1 || die "请先安装 xorriso（apt-get install xorriso isolinux syslinux-common mtools dosfstools）"
command -v sha256sum >/dev/null 2>&1 || die "缺少 sha256sum"
[ -f "${SOURCE_ISO}" ] || die "找不到源 ISO: ${SOURCE_ISO}"
[ -f "${PAYLOAD_DIR}/privacyos-server" ] || die "载荷缺少 privacyos-server，请先运行 build-payload.sh"
[ -f "${PAYLOAD_DIR}/web/index.html" ] || die "载荷缺少前端产物，请先运行 build-payload.sh"
[ -f "${PAYLOAD_DIR}/initial-admin-password" ] || die "载荷缺少 initial-admin-password"

# isohybrid MBR
MBR=""
for candidate in \
    /usr/lib/ISOLINUX/isohdpfx.bin \
    /usr/lib/syslinux/mbr/isohdpfx.bin \
    /usr/share/syslinux/isohdpfx.bin \
    /usr/lib/syslinux/isohdpfx.bin; do
    if [ -f "${candidate}" ]; then MBR="${candidate}"; break; fi
done
[ -n "${MBR}" ] || die "找不到 isohdpfx.bin（请安装 isolinux / syslinux-common）"

SOURCE_SIZE="$(stat -c%s "${SOURCE_ISO}")"
log "源 ISO : ${SOURCE_ISO} ($(numfmt --to=iec "${SOURCE_SIZE}" 2>/dev/null || echo "${SOURCE_SIZE} bytes"))"
log "载荷   : ${PAYLOAD_DIR}"
log "工作目录: ${WORK_DIR}"
log "输出   : ${OUTPUT_ISO}"
[ "${NETINST}" = "1" ] && log "模式   : netinst（安装时使用在线 Debian 镜像）"

mkdir -p "${WORK_DIR}" "$(dirname "${OUTPUT_ISO}")"

# ---------------------------------------------------------------------------
# 1. 解包源 ISO
# ---------------------------------------------------------------------------
if [ ! -f "${TREE_DIR}/.disk/info" ]; then
    log "解包源 ISO（约 4GB，需要几分钟）…"
    rm -rf "${TREE_DIR}"
    mkdir -p "${TREE_DIR}"
    xorriso -osirrox on -indev "${SOURCE_ISO}" -extract / "${TREE_DIR}" >/dev/null
    chmod -R u+w "${TREE_DIR}"
else
    log "复用已解包的 ISO 目录: ${TREE_DIR}"
fi

[ -f "${TREE_DIR}/install.amd/vmlinuz" ] || die "源 ISO 结构异常：缺少 install.amd/vmlinuz"
[ -f "${TREE_DIR}/isolinux/isolinux.bin" ] || die "源 ISO 缺少 isolinux/isolinux.bin"
[ -f "${TREE_DIR}/boot/grub/efi.img" ] || die "源 ISO 缺少 boot/grub/efi.img"

# ---------------------------------------------------------------------------
# 2. 覆盖启动菜单
# ---------------------------------------------------------------------------
log "写入 PrivacyOS 启动菜单…"
cp -f "${ROOT}/installer/boot/isolinux/"*.cfg "${TREE_DIR}/isolinux/"
cp -f "${ROOT}/installer/boot/grub/grub.cfg" "${TREE_DIR}/boot/grub/grub.cfg"
cp -f "${ROOT}/installer/boot/grub/x86_64-efi/grub.cfg" "${TREE_DIR}/boot/grub/x86_64-efi/grub.cfg"

# ---------------------------------------------------------------------------
# 3. 合并 preseed 并替换占位符
# ---------------------------------------------------------------------------
log "生成 preseed…"
ADMIN_PASSWORD="$(tr -d '\r\n' < "${PAYLOAD_DIR}/initial-admin-password")"
LUKS_PASSPHRASE="${PRIVACYOS_LUKS_PASSPHRASE:-}"
if [ -z "${LUKS_PASSPHRASE}" ]; then
    LUKS_PASSPHRASE="$(dd if=/dev/urandom bs=64 count=1 2>/dev/null | base64 | tr -dc 'A-Za-z0-9' | cut -c1-28)"
    printf '%s\n' "${LUKS_PASSPHRASE}" > "${ROOT}/build/LUKS-PASSPHRASE.txt"
    chmod 0600 "${ROOT}/build/LUKS-PASSPHRASE.txt"
    warn "已生成专业版 LUKS 口令，保存在 build/LUKS-PASSPHRASE.txt（请立即备份）"
else
    printf '%s\n' "${LUKS_PASSPHRASE}" > "${ROOT}/build/LUKS-PASSPHRASE.txt"
    chmod 0600 "${ROOT}/build/LUKS-PASSPHRASE.txt"
fi
case "${LUKS_PASSPHRASE}" in
    *[!A-Za-z0-9]*) die "PRIVACYOS_LUKS_PASSPHRASE 只能包含字母和数字" ;;
esac

install -d -m 0755 "${TREE_DIR}/preseed"
for ed in normal pro; do
    merged="${WORK_DIR}/preseed-${ed}.cfg"
    cat "${ROOT}/installer/preseed/common.cfg" > "${merged}"
    echo "" >> "${merged}"
    cat "${ROOT}/installer/preseed/${ed}.cfg" >> "${merged}"
    sed -i "s|__ADMIN_PASSWORD__|${ADMIN_PASSWORD}|g; s|__LUKS_PASSPHRASE__|${LUKS_PASSPHRASE}|g" "${merged}"
    if [ "${NETINST}" = "1" ]; then
        # A netinst image has no binary package pool: install from the online
        # Debian mirror instead of relying on the CD-ROM.
        sed -i 's|apt-setup/use_mirror boolean false|apt-setup/use_mirror boolean true|; s|apt-setup/cdrom/set-first boolean true|apt-setup/cdrom/set-first boolean false|' "${merged}"
    fi
    if grep -q '__[A-Z_]*__' "${merged}"; then
        die "preseed/${ed}.cfg 仍包含未替换的占位符"
    fi
    install -m 0644 "${merged}" "${TREE_DIR}/preseed/${ed}.cfg"
done

# ---------------------------------------------------------------------------
# 4. 复制 PrivacyOS 载荷
# ---------------------------------------------------------------------------
log "复制安装载荷到 ISO…"
rm -rf "${TREE_DIR}/privacyos"
mkdir -p "${TREE_DIR}/privacyos"
cp -a "${PAYLOAD_DIR}/." "${TREE_DIR}/privacyos/"
chmod 0755 "${TREE_DIR}/privacyos/install.sh" "${TREE_DIR}/privacyos/banner.sh"
# ISO9660 不保留 0600；安装脚本首次运行后会 shred 该文件
chmod 0644 "${TREE_DIR}/privacyos/initial-admin-password"

# ---------------------------------------------------------------------------
# 5. 品牌信息
# ---------------------------------------------------------------------------
printf 'PrivacyOS %s amd64 (Debian 13.7.0 Trixie) - %s\n' "${VERSION}" "$(date -u '+%Y%m%d-%H:%M')" \
    > "${TREE_DIR}/.disk/info"
cat > "${TREE_DIR}/privacyos/INSTALL.txt" <<EOF
PrivacyOS ${VERSION}
=====================
1. 从本 ISO 启动，选择 Normal 或 Pro 安装。
2. Pro 版全盘加密口令在构建机 build/LUKS-PASSPHRASE.txt 中。
3. 安装完成后浏览器访问 http://<设备IP>。
4. 初始管理员密码见目标系统 /root/privacyos-initial-password。
5. 联网后可选组件（Samba/NFS/Docker）会自动安装；
   也可手动执行 privacyos-provision。
EOF

# ---------------------------------------------------------------------------
# 6. md5sum
# ---------------------------------------------------------------------------
if [ "${DO_MD5}" -eq 1 ]; then
    log "重新生成 md5sum.txt（大文件较多，需要几分钟）…"
    (
        cd "${TREE_DIR}"
        find . -type f ! -name md5sum.txt -print0 | sort -z | xargs -0 md5sum > md5sum.txt
    )
fi

# ---------------------------------------------------------------------------
# 7. 打包为 BIOS+UEFI 混合 ISO
# ---------------------------------------------------------------------------
log "生成 ISO…"
rm -f "${OUTPUT_ISO}"
(
    cd "${TREE_DIR}"
    xorriso -as mkisofs \
        -r -J -joliet-long -cache-inodes \
        -isohybrid-mbr "${MBR}" \
        -partition_offset 16 \
        -V "PRIVACYOS_${VERSION//./_}" \
        -A "PrivacyOS ${VERSION} (Debian 13.7 Trixie)" \
        -p "PrivacyOS Build System" \
        -publisher "PrivacyOS" \
        -b isolinux/isolinux.bin \
        -c isolinux/boot.cat \
        -no-emul-boot -boot-load-size 4 -boot-info-table \
        -eltorito-alt-boot \
        -e boot/grub/efi.img \
        -no-emul-boot \
        -isohybrid-gpt-basdat \
        -o "${OUTPUT_ISO}" \
        .
) 2>&1 | tail -20

[ -s "${OUTPUT_ISO}" ] || die "ISO 生成失败"

# ---------------------------------------------------------------------------
# 8. 校验
# ---------------------------------------------------------------------------
log "校验引导记录…"
xorriso -indev "${OUTPUT_ISO}" -report_el_torito plain 2>/dev/null | sed 's/^/    /' || true

OUT_SIZE="$(stat -c%s "${OUTPUT_ISO}")"
log "输出大小: $(numfmt --to=iec "${OUT_SIZE}" 2>/dev/null || echo "${OUT_SIZE} bytes")"
[ "${OUT_SIZE}" -gt "${MIN_ISO_SIZE}" ] || die "输出 ISO 异常偏小，构建可能失败"

(
    cd "$(dirname "${OUTPUT_ISO}")"
    sha256sum "$(basename "${OUTPUT_ISO}")" > "$(basename "${OUTPUT_ISO}").sha256"
)

if [ "${KEEP_WORK}" -eq 0 ]; then
    log "清理工作目录 ${TREE_DIR}（使用 --keep 可保留）"
    rm -rf "${TREE_DIR}"
else
    log "已保留工作目录: ${TREE_DIR}"
fi

cat <<EOF

  ============================================================
   PrivacyOS ISO 构建完成
  ------------------------------------------------------------
   ISO      : ${OUTPUT_ISO}
   SHA256   : $(cut -d' ' -f1 "${OUTPUT_ISO}.sha256")
   管理员   : padmin / ${ADMIN_PASSWORD}
   LUKS 口令: ${ROOT}/build/LUKS-PASSPHRASE.txt
  ============================================================

  下一步：
    ./installer/scripts/verify-iso.sh "${OUTPUT_ISO}"
    使用 dd / balenaEtcher / Rufus 写入 U 盘后启动安装。

EOF
