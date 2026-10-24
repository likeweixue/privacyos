#!/bin/bash
# ============================================================================
# PrivacyOS 开机 banner
# 显示系统名称、版本、版本类型、IP、Web 面板地址与官网标识
# ============================================================================
set -u
INSTALL_DIR="/opt/privacyos"
VERSION="$(cat "${INSTALL_DIR}/version" 2>/dev/null || echo "1.3.0")"
EDITION="$(cat "${INSTALL_DIR}/edition" 2>/dev/null || echo "normal")"

if [ "${EDITION}" = "pro" ]; then
    EDITION_NAME="专业版"
    EDITION_TAG="PRO"
else
    EDITION_NAME="普通版"
    EDITION_TAG="NORMAL"
fi

# 等待网络就绪（最多 10 秒），避免开机时取不到 IP
for _ in $(seq 1 10); do
    IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
    [ -n "${IP}" ] && break
    sleep 1
done
[ -n "${IP:-}" ] || IP="127.0.0.1"
HOST="$(hostname 2>/dev/null || echo privacyos)"

BANNER=$(cat <<EOF
  ██████╗ ██████╗ ██╗██╗   ██╗ █████╗  ██████╗██╗   ██╗
  ██╔══██╗██╔══██╗██║██║   ██║██╔══██╗██╔════╝╚██╗ ██╔╝
  ██████╔╝██████╔╝██║██║   ██║███████║██║      ╚████╔╝
  ██╔═══╝ ██╔══██╗██║╚██╗ ██╔╝██╔══██║██║       ╚██╔╝
  ██║     ██║  ██║██║ ╚████╔╝ ██║  ██║╚██████╗   ██║
  ╚═╝     ╚═╝  ╚═╝╚═╝  ╚═══╝  ╚═╝  ╚═╝ ╚═════╝   ╚═╝
           PrivacyOS ${VERSION}  (${EDITION_NAME})

  主机名   : ${HOST}
  IP 地址  : ${IP}
  Web 面板 : http://${IP}
  官网标识 : privacyos
  版本类型 : ${EDITION_TAG}
  基础系统 : Debian 13.7 Trixie
  初始密码 : /root/privacyos-initial-password（首次登录后请修改）
  ------------------------------------------------------------
  提示：浏览器打开上方地址即可进入管理面板。
        专业版数据销毁机制已就绪，请谨慎配置策略。
EOF
)

# 同时更新登录提示
cat > /etc/issue <<EOF

  PrivacyOS ${VERSION} (${EDITION_NAME}) — Debian 13.7 Trixie
  Web 面板: http://${IP}    IP: ${IP}
  官网标识: privacyos

EOF

# 输出到物理控制台（存在且可写时）
if [ -w /dev/tty1 ]; then
    printf '\033[2J\033[H' > /dev/tty1 2>/dev/null || true
    printf '%s\n' "${BANNER}" > /dev/tty1 2>/dev/null || true
fi

# 同时写日志，便于 journalctl 查看
printf '%s\n' "${BANNER}"
