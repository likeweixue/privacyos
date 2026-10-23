#!/bin/bash
# PrivacyOS 每周 SMART 短自检：对全部物理磁盘下发短测试
set -u
command -v smartctl >/dev/null 2>&1 || exit 0
command -v lsblk >/dev/null 2>&1 || exit 0
logger -t privacyos-smart "开始 SMART 短测试"
while read -r name type; do
    [ "${type}" = "disk" ] || continue
    dev="/dev/${name}"
    case "${dev}" in
        /dev/loop*|/dev/ram*|/dev/sr*) continue ;;
    esac
    smartctl -t short "${dev}" >/dev/null 2>&1 && logger -t privacyos-smart "已下发短测试: ${dev}"
done < <(lsblk -dno NAME,TYPE)
