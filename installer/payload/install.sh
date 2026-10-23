#!/bin/bash
# ============================================================================
# PrivacyOS 安装脚本
#
# 运行位置：
#   * 由 preseed late_command 在目标系统 chroot 内调用
#   * 也可在已启动的 Debian 13.7 上重复执行（幂等升级部署）
#
# 设计原则：
#   1. 只做文件部署、账号、systemd enable —— 不在 chroot 中 start 服务
#   2. 可选组件（Samba/NFS/Docker）由首次启动的 provision 服务在线补齐，
#      离线安装不会失败，面板会自动降级
#   3. 初始管理员密码在构建 ISO 时随机生成，安装后写入 0600 文件
# ============================================================================
set -Eeuo pipefail

SRC="/opt/privacyos"
INSTALL_DIR="/opt/privacyos"
DATA_DIR="/data"
CONFIG_DIR="/etc/privacyos"
LOG_DIR="/var/log/privacyos"
STATE_DIR="/var/lib/privacyos"
VERSION="$(cat "${SRC}/version" 2>/dev/null || echo "1.2.1")"
EDITION="$(cat "${SRC}/edition" 2>/dev/null || echo "normal")"
[ "${EDITION}" = "pro" ] || EDITION="normal"
START_SERVICES=0
FORCE_PROVISION=0

for arg in "$@"; do
    case "${arg}" in
        --start) START_SERVICES=1 ;;
        --provision-now) FORCE_PROVISION=1 ;;
        *) ;;
    esac
done

log()  { printf '\033[1;32m[PrivacyOS]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[PrivacyOS]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m[PrivacyOS]\033[0m %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "必须以 root 运行"

if [ "${EDITION}" = "pro" ]; then
    EDITION_NAME="专业版"
else
    EDITION_NAME="普通版"
fi

log "开始安装 PrivacyOS v${VERSION} (${EDITION_NAME})"

# ---------------------------------------------------------------------------
# 1. 目录结构
# ---------------------------------------------------------------------------
install -d -m 0755 "${INSTALL_DIR}" "${INSTALL_DIR}/bin" "${INSTALL_DIR}/web" "${INSTALL_DIR}/config"
install -d -m 0755 "${DATA_DIR}" "${DATA_DIR}/users" "${DATA_DIR}/photos" "${DATA_DIR}/movies" \
    "${DATA_DIR}/backups" "${DATA_DIR}/docker" "${DATA_DIR}/trash"
install -d -m 0700 "${DATA_DIR}/vaults" "${DATA_DIR}/.cache"
install -d -m 0750 "${CONFIG_DIR}" "${CONFIG_DIR}/vault-keys"
install -d -m 0750 "${LOG_DIR}"
install -d -m 0755 "${STATE_DIR}"

# 普通版不隔离用户；专业版把每个用户目录收紧到 0700。
if [ "${EDITION}" = "pro" ]; then
    chmod 0711 "${DATA_DIR}/users" "${DATA_DIR}/photos" "${DATA_DIR}/movies" 2>/dev/null || true
    chmod 0700 "${CONFIG_DIR}" "${DATA_DIR}/vaults" "${CONFIG_DIR}/vault-keys" 2>/dev/null || true
else
    chmod 0755 "${DATA_DIR}/users" "${DATA_DIR}/photos" "${DATA_DIR}/movies" 2>/dev/null || true
fi

# ---------------------------------------------------------------------------
# 2. 部署程序与前端
# ---------------------------------------------------------------------------
if [ -f "${SRC}/privacyos-server" ]; then
    install -m 0755 "${SRC}/privacyos-server" "${INSTALL_DIR}/bin/privacyos-server"
elif [ -f "${SRC}/bin/privacyos-server" ]; then
    install -m 0755 "${SRC}/bin/privacyos-server" "${INSTALL_DIR}/bin/privacyos-server"
else
    die "载荷中缺少 privacyos-server 二进制"
fi

if [ -d "${SRC}/web" ] && [ -n "$(ls -A "${SRC}/web" 2>/dev/null)" ]; then
    if [ "${SRC}" = "${INSTALL_DIR}" ]; then
        # 载荷本身位于安装目录：直接覆盖，避免先删除源目录
        cp -a "${SRC}/web/." "${INSTALL_DIR}/web/"
    else
        rm -rf "${INSTALL_DIR}/web"
        cp -a "${SRC}/web" "${INSTALL_DIR}/web"
    fi
    chmod -R a+rX "${INSTALL_DIR}/web"
    log "已部署 Web 前端"
else
    warn "载荷中缺少 Web 前端，面板将显示占位页面（请检查构建流程）"
fi

if [ -f "${SRC}/banner.sh" ]; then
    install -m 0755 "${SRC}/banner.sh" "${INSTALL_DIR}/banner.sh"
fi
if [ -f "${SRC}/privacyos-provision.sh" ]; then
    install -m 0755 "${SRC}/privacyos-provision.sh" /usr/local/sbin/privacyos-provision
fi
if [ -f "${SRC}/privacyos-smart-scan.sh" ]; then
    install -m 0755 "${SRC}/privacyos-smart-scan.sh" /usr/local/sbin/privacyos-smart-scan
fi

printf '%s\n' "${VERSION}" > "${INSTALL_DIR}/version"
printf '%s\n' "${EDITION}" > "${INSTALL_DIR}/edition"
chmod 0644 "${INSTALL_DIR}/version" "${INSTALL_DIR}/edition"

# ---------------------------------------------------------------------------
# 3. 管理员初始密码（仅首次安装时生成/读取）
# ---------------------------------------------------------------------------
ENV_FILE="${CONFIG_DIR}/privacyos.env"
INITIAL_FILE="${SRC}/initial-admin-password"
ADMIN_PASSWORD=""

if [ -f "${ENV_FILE}" ] && grep -q '^PRIVACYOS_ADMIN_PASSWORD=' "${ENV_FILE}" 2>/dev/null; then
    ADMIN_PASSWORD="$(sed -n 's/^PRIVACYOS_ADMIN_PASSWORD=//p' "${ENV_FILE}" | head -1)"
    log "检测到已有配置，保留现有管理员密码"
elif [ -s "${INITIAL_FILE}" ]; then
    ADMIN_PASSWORD="$(tr -d '\r\n' < "${INITIAL_FILE}")"
else
    # 32 位字母数字，避免 shell/INI/环境文件转义问题
    ADMIN_PASSWORD="$(dd if=/dev/urandom bs=64 count=1 2>/dev/null | base64 | tr -dc 'A-Za-z0-9' | cut -c1-32)"
    log "未提供初始密码文件，已随机生成"
fi

[ -n "${ADMIN_PASSWORD}" ] || die "无法确定管理员初始密码"

umask 077
cat > "${ENV_FILE}" <<EOF
# PrivacyOS 运行时环境（0600，仅 root 可读）
PRIVACYOS_EDITION=${EDITION}
PRIVACYOS_LISTEN=:80
PRIVACYOS_DATA_DIR=${DATA_DIR}
PRIVACYOS_CONFIG_DIR=${CONFIG_DIR}
PRIVACYOS_LOG_DIR=${LOG_DIR}
PRIVACYOS_ADMIN_USER=padmin
PRIVACYOS_ADMIN_PASSWORD=${ADMIN_PASSWORD}
EOF
chmod 0600 "${ENV_FILE}"

cat > /root/privacyos-initial-password <<EOF
PrivacyOS 初始管理员账号
----------------------------------------
用户名: padmin
密码  : ${ADMIN_PASSWORD}

首次登录后请立即在「系统设置 → 账户安全」中修改密码。
本文件权限为 0600，可在确认新密码可用后删除：
    shred -u /root/privacyos-initial-password
EOF
chmod 0600 /root/privacyos-initial-password
umask 022

# 删除 ISO 载荷中的明文初始密码
if [ -f "${INITIAL_FILE}" ]; then
    shred -u "${INITIAL_FILE}" 2>/dev/null || rm -f "${INITIAL_FILE}"
fi

# ---------------------------------------------------------------------------
# 4. 用户、组与目录权限
# ---------------------------------------------------------------------------
groupadd -f users 2>/dev/null || true
if id padmin >/dev/null 2>&1; then
    usermod -aG users padmin 2>/dev/null || true
    usermod -s /bin/bash padmin 2>/dev/null || true
    printf 'padmin:%s\n' "${ADMIN_PASSWORD}" | chpasswd
    for d in "${DATA_DIR}/users/padmin" "${DATA_DIR}/photos/padmin" "${DATA_DIR}/movies/padmin"; do
        install -d -m 0700 "$d"
        chown -R padmin:padmin "$d" 2>/dev/null || true
    done
    if [ "${EDITION}" = "pro" ]; then
        chmod 0700 "${DATA_DIR}/users/padmin"
    else
        chmod 0755 "${DATA_DIR}/users/padmin"
    fi
else
    warn "未找到 padmin 用户（可能不是通过官方安装器安装），请手动创建管理员账号"
fi

# ---------------------------------------------------------------------------
# 5. systemd 单元
# ---------------------------------------------------------------------------
for unit in "${SRC}"/*.service "${SRC}"/*.timer; do
    [ -f "${unit}" ] || continue
    install -m 0644 "${unit}" "/etc/systemd/system/$(basename "${unit}")"
done

# 专业版需要 cryptsetup；普通版也保留，便于后续升级
if [ ! -f /etc/systemd/system/privacyos.service ]; then
    die "缺少 privacyos.service 单元文件"
fi

# ---------------------------------------------------------------------------
# 6. Samba / NFS / 防火墙 / SSH 基础配置
# ---------------------------------------------------------------------------
if [ -f "${SRC}/smb.conf" ]; then
    install -d -m 0755 /etc/samba
    if [ ! -f /etc/samba/smb.conf ] || grep -q 'Generated by PrivacyOS' /etc/samba/smb.conf 2>/dev/null; then
        install -m 0644 "${SRC}/smb.conf" /etc/samba/smb.conf
    else
        warn "检测到已有自定义 /etc/samba/smb.conf，保留原文件（面板会覆盖为托管配置）"
    fi
fi

# NFS 导出（服务端若未安装，provision 时会再次应用）
if command -v exportfs >/dev/null 2>&1; then
    cat > /etc/exports <<EOF
# Generated by PrivacyOS
${DATA_DIR}/users 192.168.0.0/16(rw,sync,no_subtree_check,root_squash,crossmnt)
${DATA_DIR}/users 10.0.0.0/8(rw,sync,no_subtree_check,root_squash,crossmnt)
EOF
    chmod 0644 /etc/exports
fi

# SSH 加固（保留密码登录以便首次使用，禁止 root 登录）
install -d -m 0755 /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/10-privacyos.conf <<'EOF'
# Managed by PrivacyOS
PermitRootLogin no
PasswordAuthentication yes
KbdInteractiveAuthentication no
MaxAuthTries 3
LoginGraceTime 30
ClientAliveInterval 300
ClientAliveCountMax 2
AllowUsers padmin
EOF
if command -v sshd >/dev/null 2>&1; then sshd -t 2>/dev/null || warn "sshd 配置校验失败，已保留文件请手动检查"; fi

# 内核与文件系统加固
cat > /etc/sysctl.d/99-privacyos.conf <<'EOF'
# Managed by PrivacyOS
kernel.dmesg_restrict = 1
kernel.kptr_restrict = 2
kernel.yama.ptrace_scope = 1
kernel.unprivileged_bpf_disabled = 1
net.ipv4.conf.all.rp_filter = 1
net.ipv4.conf.default.rp_filter = 1
net.ipv4.tcp_syncookies = 1
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.all.send_redirects = 0
net.ipv6.conf.all.accept_redirects = 0
fs.protected_hardlinks = 1
fs.protected_symlinks = 1
fs.protected_fifos = 2
fs.protected_regular = 2
EOF
sysctl -q -p /etc/sysctl.d/99-privacyos.conf 2>/dev/null || true

# 日志轮转
cat > /etc/logrotate.d/privacyos <<EOF
${LOG_DIR}/*.log {
    weekly
    rotate 8
    compress
    delaycompress
    missingok
    notifempty
    create 0640 root adm
}
EOF

# 数据目录权限标记（供面板与备份脚本参考）
cat > "${CONFIG_DIR}/install-info.json" <<EOF
{
  "version": "${VERSION}",
  "edition": "${EDITION}",
  "installed_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "installer": "privacyos-preseed"
}
EOF
chmod 0644 "${CONFIG_DIR}/install-info.json"

# ---------------------------------------------------------------------------
# 7. 启用服务（chroot 中不启动）
# ---------------------------------------------------------------------------
systemctl daemon-reload 2>/dev/null || true
for svc in privacyos.service privacyos-banner.service privacyos-provision.service; do
    systemctl enable "${svc}" 2>/dev/null || warn "启用 ${svc} 失败（稍后可手动 systemctl enable）"
done
systemctl enable privacyos-smart.timer 2>/dev/null || true
for svc in smbd nmbd nfs-kernel-server docker; do
    if systemctl list-unit-files "${svc}.service" >/dev/null 2>&1; then
        systemctl enable "${svc}.service" 2>/dev/null || true
    fi
done

# 首次启动 provisioning（在线补齐可选组件 + 同步 Samba 密码）
if [ "${FORCE_PROVISION}" -eq 1 ] && [ "${START_SERVICES}" -eq 1 ]; then
    /usr/local/sbin/privacyos-provision --force || warn "可选组件安装失败，面板会自动降级"
fi

if [ "${START_SERVICES}" -eq 1 ]; then
    systemctl restart privacyos.service 2>/dev/null || systemctl start privacyos.service 2>/dev/null || true
fi

# ---------------------------------------------------------------------------
# 8. 结束信息
# ---------------------------------------------------------------------------
IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
[ -n "${IP}" ] || IP="<本机IP>"
cat > /etc/issue <<EOF

  PrivacyOS ${VERSION} (${EDITION_NAME}) — 基于 Debian 13.7 Trixie
  Web 面板: http://${IP}
  官网标识: privacyos

EOF

cat <<EOF

  ============================================================
   PrivacyOS v${VERSION} (${EDITION_NAME}) 安装完成
  ------------------------------------------------------------
   Web 面板 : http://${IP}
   数据目录 : ${DATA_DIR}
   配置文件 : ${CONFIG_DIR}
   初始密码 : /root/privacyos-initial-password (0600)
   首次登录后请立即修改密码并启用双重认证。
   可选组件（Samba/NFS/Docker）将在联网后自动安装，
   也可手动执行: privacyos-provision
  ============================================================

EOF

if [ "${EDITION}" = "pro" ]; then
    warn "专业版已启用全盘加密。请在构建机保存好 LUKS 安装口令，"
    warn "并尽快使用 cryptsetup luksChangeKey 更换为自己的口令。"
fi

log "完成"
