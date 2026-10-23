# 安全模型

PrivacyOS 的隐私承诺建立在三个层次上：**身份与访问控制**、**操作系统级隔离**、**可验证的销毁**。
本文说明威胁模型、已实现的控制、以及必须由部署者承担的边界。

## 1. 资产与威胁模型

### 受保护资产

- 用户文件、照片、影视与元数据
- 账号凭据、会话、2FA 密钥
- 分享链接与访问密码
- 审计/销毁日志
- LUKS 密钥与恢复密钥

### 对抗者

| 对抗者 | 能力 | 主要防线 |
| --- | --- | --- |
| 网络攻击者 | 扫描、爆破、中间人 | 锁定/退避、2FA、HTTPS、nftables、最小服务面 |
| 同机普通用户 | 已登录面板、可访问 SMB | Linux 0700 权限、路径沙箱、API 归属校验 |
| 普通版管理员 | 可读取所有用户文件 | 明确的产品语义；建议高隐私场景使用专业版 |
| 专业版管理员 | 不能读取他人文件 | 0700 + 路径沙箱 + 后端强制 |
| 窃取磁盘者 | 物理接触数据盘 | 专业版 LUKS2 全盘加密 |
| 恶意容器 | 容器逃逸/资源滥用 | Docker 隔离、资源限制（部署者配置）、非必要不开放 |
| 本地非特权进程 | 读取面板内存/配置 | systemd 加固、0600 配置、密钥文件权限 |

### 不在范围内

- 物理侧信道、冷启动内存取证、硬件后门
- 已获得 root 的攻击者（root 可读取进程内存与密钥文件）
- 供应链攻击（Debian 镜像、上游依赖）
- 云主机商对虚拟磁盘的访问

## 2. 身份认证

- 密码：bcrypt，cost 12；长度 8–72；未知用户执行 dummy hash 以降低枚举时序差异。
- 会话：JWT HS256，12 小时 access + 7 天 refresh；JWT 的 `jti` 对应 `sessions.id`，
  每次请求校验会话未被撤销、未过期。
- 2FA：TOTP（SHA1/6 位/30 秒），允许 ±1 时间窗；密钥仅存于 SQLite。
- 登录失败：
  - 按用户名 + 时间窗口统计；
  - 第 3 次起指数退避：3s、6s、12s……上限 300s；
  - 达到 `max_attempts` 临时锁定（默认 10 次 / 15 分钟）；
  - 专业版达到销毁阈值时触发异步销毁，并在响应中只返回通用错误。
- 登录接口额外做 IP 级限流（20 次/分钟）。
- 会话管理：修改密码保留当前会话、撤销其他；管理员重置密码撤销全部并同步 Linux/Samba。

### 专业版销毁阈值

销毁阈值默认 10 次，可配置 3–50。**这是不可恢复操作**，请：

1. 在启用前确认备份策略；
2. 通过 2FA 保护高价值账号；
3. 避免把管理员账号与销毁策略同时暴露在公网；
4. 在测试环境验证通知与审计。

## 3. 授权与隔离

### 角色 + 功能权限

```text
角色 admin/user
  └─ 功能权限 files/photos/movies/share/docker/apps
```

- 管理路由（用户、存储、网络、应用、日志、设置、系统）要求 `admin`。
- 功能路由在非管理员时检查 `permissions`；管理员默认全开。
- 专业版管理员不能通过 `scope=all` 或 `user=` 绕过限制（后端直接忽略）。

### 文件系统

- 每个用户主目录：普通版 0755，专业版 0700。
- `/data/users` 父目录：普通版 0755，专业版 0711（可穿越但不可列目录）。
- Samba 共享 `[users]` 指向 `/data/users`，专业版启用
  `access based share enum` 与 `hide unreadable`。
- WebDAV 只挂载当前用户 home，Basic Auth 校验失败延迟 300ms。
- API 路径解析 `SecureJoin` 拒绝 `..`、绝对路径、符号链接逃逸；
  `path` 永远不会以绝对路径出现在响应中（分享/回收站仅返回展示名）。

### 配额

配额按用户统计，防止单个用户耗尽磁盘；超出返回 507。配额检查在写临时文件前后各一次，
避免并发上传绕过。

## 4. 数据加密

### 全盘加密（专业版安装）

- Debian installer `partman-auto/method crypto`，LUKS2 + LVM。
- 口令由构建时随机生成（28 位字母数字），写入构建机 `build/LUKS-PASSPHRASE.txt`。
- 风险：**构建机若被入侵，口令泄露**；安装后应立即更换：

```bash
sudo cryptsetup luksChangeKey /dev/<device>
```

### 每用户加密卷（专业版可选）

- 每个用户一个 LUKS2 稀疏文件 `/data/vaults/<user>.luks`。
- 恢复密钥 `/etc/privacyos/vault-keys/<user>.key`（0600，仅 root）。
- 登录/解锁时 `cryptsetup open` + `mount` 到用户 home。
- 创建卷时自动把现有 home 内容迁入。
- 销毁：`umount` → `cryptsetup close` → 覆写并删除 keyfile →
  `cryptsetup luksErase`（擦除全部 keyslot）→ 删除容器文件。

### 销毁语义

| 方式 | 速度 | 可恢复性 | 说明 |
| --- | --- | --- | --- |
| `key_destroy` | 极快 | 否 | 擦除 LUKS keyslot，密文永久不可解 |
| `overwrite` | 慢 | 否 | 逐文件随机覆写 + fsync，然后删除 |
| `delete` | 快 | 可能 | 仅删除文件，不推荐用于敏感数据 |

- 无 LUKS 卷时按 `fallback` 执行（`none` / `overwrite` / `delete`）。
- 销毁同时撤销会话、禁用账号、删除分享、清理相册/影视索引。
- `silent=true` 时不通知用户；`notify_admin=true` 额外通知管理员。
- 每次销毁写入 `destroy_events` 与审计日志。

### SSD / 写时复制文件系统注意

- 覆写无法保证覆盖 SSD 的磨损均衡副本；`key_destroy` 依赖加密，是更强的保证。
- 对 Btrfs/ZFS 快照，删除文件不删除快照数据；销毁前应先清理快照。

## 5. 网络与传输

- 默认 HTTP:80。**首次登录后应在「系统设置 → 安全策略」生成自签名证书并重启**，
  或在前置反向代理上配置受信任证书。
- 自签名证书支持域名与所有本机 IP 的 SAN，有效期 10 年。
- 防火墙基于 nftables，默认放行 22/80/443/445/2049/3000/8080，策略为 drop；
  应用后 120 秒自动回滚，除非在界面确认。规则可编辑，请保留 22 端口。
- DDNS 仅更新 A 记录；Cloudflare 使用 API Token，建议最小权限（Zone.DNS Edit）。
- 出站通知（SMTP/Telegram/Bark/Server酱）会把标题与正文发送到第三方，启用前请评估。

## 6. 审计与可观测性

- 所有管理/敏感操作写入 `audit_logs`（用户、IP、目标、结果）。
- 登录成功/失败、文件操作、分享、销毁、防火墙、设置变更均可追溯。
- 审计日志只读展示；支持按条件导出 CSV。
- 日志保留 90 天；销毁事件长期保留（受清理策略约束）。
- 告警去重写入 `alert_events`，磁盘 SMART 异常会在仪表盘提示。

## 7. 系统加固

安装脚本与 systemd 单元包含：

- 关闭 root SSH 登录，限制 `AllowUsers padmin`，`MaxAuthTries 3`；
- sysctl：`dmesg_restrict`、`kptr_restrict`、`ptrace_scope`、`rp_filter`、
  `syncookies`、`protected_*` 等；
- 服务只读挂载 `/usr`、`/boot`（`ProtectSystem=yes`），`/etc` 保留可写以支持
  Samba/NFS/网络/账号配置与系统更新；`ReadWritePaths` 限定数据/配置/日志目录；
- `PrivateTmp`、`ProtectClock`、`ProtectHostname`、`RestrictSUIDSGID`、
  `LockPersonality`、`RestrictRealtime`、`RestrictAddressFamilies`；
- 可选组件（Samba/NFS/Docker）在首次联网后安装，减少默认攻击面；
- 每周 SMART 短测试，结果可在界面查看。

## 8. 已知限制

1. **面板以 root 运行**：需要挂载、cryptsetup、systemctl 等权限；systemd 加固降低但不消除风险。
2. **Docker 等价于 root**：能管理 Docker 的用户/容器可挂载宿主目录；仅向可信用户开放该权限。
3. **Samba/NFS 明文协议**：局域网使用；跨公网请用 VPN/HTTPS。
4. **Basic Auth over HTTP**：WebDAV 在未启用 HTTPS 时会明文传输凭据。
5. **专业版管理员仍可执行销毁**：这是产品能力，不是提权；但需配合审计与备份。
6. **恢复密钥在系统中**：`/etc/privacyos/vault-keys` 由 root 持有，
   能读取 root 文件系统的攻击者可解锁卷；真正的"无人值守加密"需要外部 KMS/TPM，
   当前版本未实现。
7. **安装器随机口令**：构建机保存了 LUKS 口令与初始 Web 密码；
   构建产物应按敏感介质管理。
8. **未做模糊测试/渗透测试**：生产部署前建议进行独立安全评估。

## 9. 安全事件响应

1. 断开网络或启用防火墙（保留 22 端口）。
2. 在「日志与审计」导出最近审计与登录记录。
3. 重置相关账号密码，注销全部会话，检查 2FA。
4. 如怀疑 LUKS 口令泄露：更换 `luksChangeKey`，并考虑重建加密卷。
5. 如确认数据泄露且需要销毁：使用专业版手动销毁，保留 `destroy_events` 证据。
6. 升级到最新版本，检查 `/var/log/privacyos`、`journalctl -u privacyos`。

## 10. 漏洞报告

本项目为示例/自托管产品。发现问题请附：

- 版本与版控（`/api/v1/system/public`）
- 复现步骤与影响
- 是否已在生产环境部署

在修复前请勿公开细节。
