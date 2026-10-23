# PrivacyOS 架构设计

## 1. 设计目标

PrivacyOS 的目标不是"把一堆开源软件塞进 Debian"，而是提供一套**可交付、可验证、可解释**的 NAS 产品：

1. **单二进制后端**：目标机不需要安装 Go/Python 运行时，升级只替换一个文件。
2. **默认离线可装**：安装阶段只依赖 DVD 内的软件包；可选组件联网后补齐。
3. **隐私优先**：专业版通过 Linux 权限 + LUKS + 可审计的销毁流程实现"管理员也看不到别人数据"。
4. **可开发**：在 macOS/Windows 上无需真实磁盘、Docker、LUKS 即可运行完整面板（模拟层）。
5. **可测试**：关键安全逻辑（锁定、退避、销毁、路径沙箱）均有单元测试。

非目标（当前版本）：集群/高可用、多租户计费、人脸识别、实时转码农场。

## 2. 分层架构

```text
┌────────────────────────────────────────────────────────────┐
│ 用户访问层                                                   │
│ 浏览器（PC/手机） · SMB/CIFS · NFS · WebDAV · SSH            │
└───────────────────────────┬────────────────────────────────┘
                            │
┌───────────────────────────▼────────────────────────────────┐
│ 管理界面（Vue 3）：固定顶栏 · 侧边导航 · 可搜索下拉 · Widget    │
│ 12 个管理模块：仪表盘/文件/相册/影视/Docker/用户/存储/网络/   │
│ 应用/监控/日志/设置 + 登录页 + 公开分享页                      │
└───────────────────────────┬────────────────────────────────┘
                            │ REST /api/v1 · SSE /monitor/stream
┌───────────────────────────▼────────────────────────────────┐
│ API 层（Gin）：认证中间件 → 权限/版控 → 参数校验 → 审计       │
├────────────────────────────────────────────────────────────┤
│ 领域服务                                                     │
│ auth · files · media(photos/movies) · storage · shares        │
│ dockerx · monitor · system · core(policy/vault/jobs/notify)   │
├────────────────────────────────────────────────────────────┤
│ 数据层：SQLite(WAL) + 文件系统 + systemd/CLI/Docker Socket    │
└───────────────────────────┬────────────────────────────────┘
                            │
┌───────────────────────────▼────────────────────────────────┐
│ Debian 13.7 Trixie                                           │
│ 内核 6.12 · systemd 257 · Samba · NFS · Docker · mdadm        │
│ SMART · LUKS2 · nftables · ffmpeg · exiftool(可选)            │
└────────────────────────────────────────────────────────────┘
```

## 3. 后端包结构

| 包 | 职责 | 关键点 |
| --- | --- | --- |
| `cmd/privacyos-server` | 进程入口 | 配置加载、优雅退出、DDNS 循环、启动 banner |
| `internal/config` | 路径/端口/版控解析 | 开发模式自动落到 `./dev-data`；环境变量覆盖 |
| `internal/store` | SQLite schema、迁移、CRUD | WAL、外键、嵌入式 SQL 迁移、首次种子数据 |
| `internal/auth` | JWT、bcrypt、TOTP | HS256、短期 access + 可撤销 session |
| `internal/core` | 应用上下文、版控规则、策略引擎、任务、通知、加密卷 | 登录退避、销毁流程、LUKS 生命周期、janitor |
| `internal/files` | 文件管理 | 路径沙箱、配额、回收站、zip/tar、缩略图 |
| `internal/media` | 相册/影视 | EXIF、时间轴、ffprobe、TMDB、字幕转换 |
| `internal/storage` | 磁盘/RAID/SMART | lsblk JSON、smartctl JSON、mdadm |
| `internal/shares` | Samba/NFS 配置生成与应用 | 纯函数渲染 + testparm 校验 |
| `internal/dockerx` | Docker Engine API | Unix Socket HTTP、日志解复用、Compose CLI |
| `internal/monitor` | 实时指标 | /proc 采样、SSE、进程快照与结束 |
| `internal/system` | 主机信息/网络/更新/备份/防火墙 | 自签证书、nftables 回滚、DDNS、Linux 账号 |
| `internal/thumb` | 缩略图 | 图片缩放、EXIF 方向、ffmpeg 视频抽帧、磁盘缓存 |
| `internal/api` | 路由与处理器 | Gin 中间件、统一错误、SSE、WebDAV、SPA 回退 |
| 前端 `src/layouts` | 管理壳 | AdminLayout：固定顶栏、侧栏/Drawer、命令面板、会话菜单 |
| 前端 `src/apps.js` | 应用注册表 | 分组导航、图标、功能开关状态、扩展列表 |

## 4. 认证与会话

```text
POST /auth/login
  ├─ 限流（IP，20 次/分钟）
  ├─ 登录门禁：账号锁定 / 指数退避（失败 3 次起 3s、6s、12s…）
  ├─ bcrypt 校验；未知用户执行 dummy compare 以对齐耗时
  ├─ 若启用 2FA：TOTP 校验（允许 ±1 个时间窗）
  ├─ 创建 sessions 行（可撤销）
  └─ 签发 access(12h) + refresh(7d) JWT，JTI=session id

受保护请求
  ├─ Authorization: Bearer <access>（下载/SSE 支持 ?access_token=）
  ├─ 校验签名、iss、过期、type=access
  ├─ 查 users/status、sessions/revoked
  └─ 每 5 分钟更新 last_seen
```

- 密码修改：保留当前会话，撤销其他会话。
- 管理员重置密码：撤销目标全部会话并同步 Linux/Samba 密码。
- 会话列表展示 IP/UA/最近活动，可单独注销。

## 5. 版控与权限

`config.Edition` 只有 `normal` / `pro`，由安装器写入 `/opt/privacyos/edition`，
也可用 `PRIVACYOS_EDITION` 覆盖。

| 规则 | 普通版 | 专业版 |
| --- | --- | --- |
| `CanSeeOtherFiles(admin)` | 取决于 `normal.admin_see_all`（默认 true） | 恒 false |
| 用户主目录权限 | 0755 | 0700 |
| 共享路径解析 | 管理员可 `scope=all` | 永远限制在本人根目录 |
| 配额可见性 | 管理员可见 | 他人配额返回 `quota_hidden` |
| 销毁策略 | API 拒绝 | 启用 |
| 删除用户 | 立即删除数据 | 标记 `pending_deletion`，30 天 janitor 清理 |

权限分两层：

1. **角色**：`admin` / `user`，控制用户管理、存储、网络、日志、设置等管理路由。
2. **功能权限**：`files/photos/movies/share/docker/apps` 存在 `users.permissions` JSON 中；
   管理员默认全开，普通用户可按需开通。

## 6. 文件沙箱

```text
/data/users/<user>/            ← 普通用户根
/data/users/                   ← 管理员 scope=all 根（仅普通版）
/data/photos/<user>/YYYY/MM/DD/
/data/movies/<user>/
/data/trash/<user>/<uuid>/<name>
/data/vaults/<user>.luks       ← 专业版每用户 LUKS 容器
```

`files.SecureJoin(root, rel)` 的防线：

1. 去掉前导 `/`，统一分隔符；
2. 拒绝任何 `..` 段；
3. `filepath.Clean` 后拼接，检查结果仍在 root 下；
4. 对已存在路径 `EvalSymlinks`，并校验真实路径仍在 root 内；
5. 新路径则校验父目录的真实路径。

上传写入临时文件后 `fsync + rename`，按配额二次校验；删除先移动到回收站，
保留期默认 30 天，到期由 janitor 永久清理。

## 7. 相册与影视数据流

```text
照片上传/扫描
  → 读取 EXIF（拍摄时间、相机、镜头、GPS、方向）
  → 归入 /photos/<user>/YYYY/MM/DD/
  → images 表（含宽高、SHA256）
  → 缩略图按 (路径, mtime, 尺寸) 哈希缓存
  → 时间轴 = substr(taken_at,1,10) 聚合

影片扫描
  → 解析文件名（SxxExx / 年份 / 清晰度标签）
  → ffprobe 读取时长/分辨率/编码
  → movies 表
  → TMDB 搜索 + 详情 + 海报缓存（后台 Job，可取消/进度）
  → 播放：http.ServeContent 支持 Range；外挂 SRT 转 WebVTT
```

## 8. 监控

- 后端每 2 秒采样 `/proc/stat`、`/proc/meminfo`、`/proc/net/dev`、`/proc/diskstats`、
  `/proc/[pid]/*`、`/sys/class/hwmon`。
- 前端通过 `EventSource('/api/v1/monitor/stream?access_token=…')` 接收 `metrics` 事件；
  断线自动回退到轮询 `/monitor/overview`。
- 内存中保留最近 300 个样本用于仪表盘/监控图表。
- 开发模式生成平滑的正弦波数据，避免在 macOS 上读取 `/proc`。

## 9. 销毁策略状态机

```text
密码错误（专业版，策略启用）
   │
   ├─ 失败计数达到 threshold
   ▼
TriggerDestroy(username, trigger)
   │  创建 Job（异步，可在任务面板查看）
   ▼
ExecuteDestroy
   ├─ method=key_destroy 且存在 vault → DestroyVault
   │      umount → close mapper → 覆写并删除 keyfile → luksErase → 删除容器
   ├─ method=overwrite → 遍历用户数据写随机数 → 删除
   ├─ method=delete → 直接删除
   ├─ 无 vault 时按 fallback 执行
   ├─ 撤销会话、禁用账号、清理登录失败记录与分享
   ├─ 写入 destroy_events（running → completed/failed）
   └─ silent=false 或 notify_admin=true 时发送通知
   ▼
用户数据不可恢复（LUKS 密钥已擦除）
```

销毁只作用于**该用户**：`/data/users/<u>`、`/data/photos/<u>`、`/data/movies/<u>`
与回收站条目。`scope=global` 预留给未来设备级销毁，当前 API 只实现 user。

## 10. 部署目录

```text
/opt/privacyos/
├── bin/privacyos-server
├── web/                     前端静态文件
├── banner.sh
├── version / edition
/etc/privacyos/
├── privacyos.db             SQLite
├── privacyos.env            0600，含初始 admin 密码
├── secret.key               JWT 签名密钥
├── vault-keys/<user>.key    LUKS 恢复密钥（0600）
└── tls/                     HTTPS 证书
/data/                       users/photos/movies/backups/docker/trash/vaults
/var/log/privacyos/          access/auth/destroy/system（预留）+ provision.log
/var/lib/privacyos/          provisioned 标记
```

## 11. 扩展点

- **新增后端模块**：在 `internal/<module>` 实现服务 → 在 `api/server.go` 注册路由 →
  在 `frontend/src/api/index.js` 增加客户端方法。
- **新增页面**：`frontend/src/views/*.vue` + 路由元数据 + `frontend/src/apps.js` 注册表。
- **新增版控规则**：集中在 `core.App` 方法（`AdminSeeAll` / `FeatureAllowed`），
  不要在 handler 内散落 `if edition == ...`。
- **新增通知渠道**：`core/notify.go` 增加 sendXxx，并扩展 `NotificationSettings`。
- **替换数据库**：`store` 接口目前直接依赖 `database/sql`；迁移 PostgreSQL 需要替换
  占位符与自增语法，schema 已尽量使用标准 SQL。
