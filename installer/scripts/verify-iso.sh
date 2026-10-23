#!/bin/bash
# ============================================================================
# 校验 PrivacyOS ISO：引导记录、preseed、载荷与占位符
# 用法: ./installer/scripts/verify-iso.sh path/to/privacyos.iso
# ============================================================================
set -Eeuo pipefail

ISO="${1:-}"
[ -n "${ISO}" ] || { echo "用法: $0 <iso>"; exit 2; }
[ -f "${ISO}" ] || { echo "找不到 ISO: ${ISO}"; exit 2; }
command -v xorriso >/dev/null 2>&1 || { echo "需要 xorriso"; exit 2; }

TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

fail=0
ok()   { printf '  \033[1;32mOK\033[0m  %s\n' "$*"; }
bad()  { printf '  \033[1;31mFAIL\033[0m %s\n' "$*"; fail=1; }

echo "== ISO 基本信息 =="
ls -lh "${ISO}" | sed 's/^/  /'
echo "== El Torito 引导记录 =="
if xorriso -indev "${ISO}" -report_el_torito plain 2>/dev/null | tee "${TMP}/torito.txt" | sed 's/^/  /'; then
    grep -qi 'isolinux.bin' "${TMP}/torito.txt" && ok "BIOS 引导 (isolinux.bin)" || bad "缺少 BIOS 引导记录"
    grep -qi 'efi.img' "${TMP}/torito.txt" && ok "UEFI 引导 (efi.img)" || bad "缺少 UEFI 引导记录"
else
    bad "无法读取 El Torito 记录"
fi

echo "== 关键文件 =="
mkdir -p "${TMP}/out"
for dir in preseed privacyos isolinux boot/grub; do
    xorriso -osirrox on -indev "${ISO}" -extract "/${dir}" "${TMP}/out" >/dev/null 2>&1 || true
done
for f in preseed/normal.cfg preseed/pro.cfg privacyos/privacyos-server privacyos/web/index.html \
         privacyos/install.sh privacyos/banner.sh privacyos/privacyos.service; do
    if [ -f "${TMP}/out/${f}" ]; then ok "${f}"; else bad "缺少 ${f}"; fi
done

echo "== preseed 占位符 =="
if grep -rq '__ADMIN_PASSWORD__\|__LUKS_PASSPHRASE__' "${TMP}/out/preseed" 2>/dev/null; then
    bad "preseed 中仍有未替换的占位符"
else
    ok "占位符已替换"
fi
if grep -q 'partman-auto/method string crypto' "${TMP}/out/preseed/pro.cfg" 2>/dev/null; then
    ok "专业版包含 LUKS 配置"
else
    bad "专业版缺少 crypto 分区配置"
fi
if grep -q 'preseed/file=/cdrom/preseed/pro.cfg' "${TMP}/out/isolinux"/*.cfg 2>/dev/null; then
    ok "启动菜单引用专业版 preseed"
else
    bad "启动菜单未引用专业版 preseed"
fi

echo "== 载荷二进制 =="
if [ -f "${TMP}/out/privacyos/privacyos-server" ]; then
    file "${TMP}/out/privacyos/privacyos-server" | sed 's/^/  /'
    case "$(file -b "${TMP}/out/privacyos/privacyos-server")" in
        *ELF*64-bit* ) ok "linux/amd64 ELF 二进制" ;;
        *) bad "二进制格式不是 linux/amd64 ELF" ;;
    esac
fi

echo "== 校验和 =="
if [ -f "${ISO}.sha256" ]; then
    ( cd "$(dirname "${ISO}")" && sha256sum -c "$(basename "${ISO}.sha256")" ) && ok "SHA256 匹配" || bad "SHA256 不匹配"
else
    echo "  (未提供 .sha256 文件)"
fi

if [ "${fail}" -ne 0 ]; then
    echo "校验失败" >&2
    exit 1
fi
echo "全部检查通过"
