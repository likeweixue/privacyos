# PrivacyOS REST API

基础路径：`/api/v1`。除公开接口外，均需要：

```http
Authorization: Bearer <access_token>
```

浏览器中无法设置 Header 的场景（`<img>`、`<video>`、SSE、下载链接）允许使用
`?access_token=<access_token>` 查询参数。

## 1. 约定

### 成功响应

```json
{ "data": { } }
```

列表接口：

```json
{
  "data": [ ],
  "pagination": { "total": 128, "page": 1, "page_size": 50 }
}
```

### 错误响应

```json
{
  "error": {
    "code": "invalid_credentials",
    "message": "用户名或密码错误"
  }
}
```

| HTTP | 典型 code | 含义 |
| --- | --- | --- |
| 400 | `bad_request` / `invalid_path` / `confirm_required` | 参数或确认词不正确 |
| 401 | `unauthorized` / `invalid_credentials` / `totp_required` | 未登录 / 凭证错误 / 需要 2FA |
| 403 | `forbidden` / `feature_disabled` / `account_disabled` | 权限不足 / 功能未开通 / 账号停用 |
| 404 | `not_found` | 资源不存在（含跨用户访问） |
| 409 | `exists` / `user_exists` / `vault_exists` | 冲突 |
| 413/507 | `quota_exceeded` | 配额不足 |
| 429 | `too_many_attempts` / `rate_limited` | 登录退避或限流 |
| 500 | `*_failed` | 服务端错误 |

### 分页参数

`page`（默认 1）、`page_size`（默认 20–60，最大 500）。列表接口在 `pagination` 返回总数。

## 2. 认证

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/auth/login` | `{username,password,totp?}` → `{access_token,refresh_token,expires_in,user}` |
| POST | `/auth/refresh` | `{refresh_token}` → 新的令牌对 |
| POST | `/auth/logout` | 撤销当前会话 |
| GET | `/auth/me` | 当前用户、版控、功能权限、品牌信息 |
| POST | `/auth/password` | `{old_password,new_password}`；保留当前会话，撤销其他 |
| GET | `/auth/sessions` | 当前账号的会话列表 |
| DELETE | `/auth/sessions/:id` | 注销指定会话 |
| POST | `/auth/2fa/setup` | 生成 `{secret,otpauth_url}`（尚未启用） |
| POST | `/auth/2fa/enable` | `{code}` 校验后启用 |
| POST | `/auth/2fa/disable` | `{password,code}` |
| PATCH | `/auth/preferences` | `{lang,theme}` |

登录失败响应会附带：

```json
{ "error": {...}, "retry_after": 12, "locked": false, "failures": 4 }
```

前端应根据 `retry_after` 显示倒计时并禁用提交按钮。

## 3. 系统公开信息

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/healthz` | 进程存活检查（无鉴权） |
| GET | `/system/public` | 登录页品牌：名称、版本、版控、官网、主机名、IP、是否开发模式 |

## 4. 文件管理

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/files?path=&scope=all&user=` | 目录列表（entries、usage、can_see_all） |
| GET | `/files/stat?path=` | 单条元数据 |
| GET | `/files/users` | 管理员可浏览的用户列表（普通版） |
| POST | `/files/mkdir` | `{path,name}` |
| POST | `/files/rename` | `{path,name}` |
| POST | `/files/move` | `{paths:[],destination}` |
| POST | `/files/copy` | 同上 |
| POST | `/files/delete` | `{paths:[]}` → 移入回收站 |
| POST | `/files/upload` | `multipart/form-data`：`path` + `files[]`（流式、64GB 上限） |
| GET | `/files/download?path=` | 下载，支持 Range 与中文文件名 |
| GET | `/files/archive?path=a&path=b` | 打包为 zip 下载 |
| POST | `/files/extract` | `{path,destination}`，支持 zip/tar/tar.gz |
| GET | `/files/search?q=&limit=` | 名称搜索（最多 8 层） |
| GET | `/files/thumb?path=&size=` | 图片/视频缩略图（JPEG） |
| GET | `/files/trash` | 回收站列表 |
| POST | `/files/trash/restore` | `{ids:[]}` |
| POST | `/files/trash/empty` | `{ids:[]}`，空数组表示全部 |
| GET | `/files/usage` | 已用/配额/回收站占用/文件数 |
| POST | `/files/share` | `{path,root?,name?,password?,expires_hours?,max_downloads?}` |

`root` 可为 `home`（默认）、`photos`、`movies`，用于为媒体文件创建分享。

### 分享链接

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/shares?all=1` | 我的分享；管理员 `all=1` 查看全部 |
| GET | `/shares/:id` | 单条 |
| DELETE | `/shares/:id` | 撤销 |
| GET | `/s/:token` | **公开**：名称、类型、大小、是否需要密码、下载次数 |
| POST | `/s/:token/verify` | **公开**：`{password}`，成功后设置 HttpOnly 授权 Cookie |
| GET | `/s/:token/download` | **公开**：文件直链或目录 zip |

分享支持有效期与最大下载次数；密码使用 bcrypt 存储，授权 Cookie 为 HMAC 签名、2 小时有效。

## 5. 相册

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/photos?page=&page_size=&q=&from=&to=&album=` | 照片分页 |
| GET | `/photos/timeline` | 年 → 月 → 日聚合 |
| GET | `/photos/map` | 带 GPS 的照片（最多 500） |
| GET | `/photos/:id` | 详情（EXIF、尺寸、GPS） |
| GET | `/photos/:id/file` | 原图（支持 Range） |
| GET | `/photos/:id/thumb?size=` | 缩略图 |
| POST | `/photos/upload` | `multipart/form-data`: `files[]` |
| POST | `/photos/scan` | 扫描 `/data/photos/<user>`（返回 Job） |
| POST | `/photos/:id/share` | `{password?,expires_hours?}` |
| DELETE | `/photos/:id` | 删除文件与索引 |

相册：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/photos/albums` | 列表（含照片数、封面 ID） |
| POST | `/photos/albums` | `{name,description}` |
| GET | `/photos/albums/:id` | `{album,photos}` |
| DELETE | `/photos/albums/:id` | 删除相册（不删照片） |
| POST | `/photos/albums/:id/photos` | `{photo_ids:[]}` |
| DELETE | `/photos/albums/:id/photos/:photoId` | 移出相册 |

## 6. 影视中心

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/movies?q=&kind=&year=&genre=&sort=&page=` | 影片列表（poster_url/stream_url/playable） |
| GET | `/movies/meta` | 年份、类型计数、总数、TMDB 配置状态 |
| GET | `/movies/:id` | 详情 + 外挂字幕列表 |
| PATCH | `/movies/:id` | 手工编辑标题/年份/类型/简介/标签 |
| DELETE | `/movies/:id?delete_file=1` | 从库中移除，可选删除文件 |
| POST | `/movies/scan` | 扫描媒体库（Job） |
| POST | `/movies/scrape` | `{ids:[]}`，空数组表示全部未刮削（Job） |
| GET | `/movies/:id/stream` | Range 流式播放；`HEAD` 同样支持 |
| GET | `/movies/:id/poster?size=` | 本地海报缓存 |
| GET | `/movies/:id/subtitles` | 字幕列表 |
| GET | `/movies/:id/subtitles/:name` | 字幕内容（SRT 自动转 WebVTT） |
| GET | `/movies/tmdb` | 管理员：TMDB 配置（Key 脱敏） |
| PUT | `/movies/tmdb` | 管理员：`{api_key,language}`，传 `__keep__` 保留旧值 |

## 7. Docker

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/docker/status` | 守护进程版本/可用性 |
| GET | `/docker/containers?all=&stats=1` | 容器列表，可附带 CPU/内存 |
| POST | `/docker/containers/:id/:action` | `start`/`stop`/`restart`/`kill`/`pause`/`unpause` |
| DELETE | `/docker/containers/:id?force=1` | 删除容器 |
| GET | `/docker/containers/:id/logs?tail=` | 日志（自动解复用 Docker 流头） |
| GET | `/docker/containers/:id/stats` | 单次资源采样 |
| GET | `/docker/images` | 镜像列表 |
| POST | `/docker/images/pull` | `{image}` → Job |
| DELETE | `/docker/images/:id` | 删除镜像 |
| GET | `/docker/compose` | Compose 项目列表 |
| POST | `/docker/compose` | `{name,content}` 保存 docker-compose.yml |
| POST | `/docker/compose/:name/up` | 部署 |
| POST | `/docker/compose/:name/down` | 停止 |
| GET | `/docker/compose/:name/logs?tail=` | 项目日志 |
| DELETE | `/docker/compose/:name` | 停止并删除项目（保留数据目录） |

## 8. 存储管理（管理员）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/storage/disks` | 磁盘 + SMART 健康 + 温度 + 用量 |
| GET | `/storage/smart?device=/dev/sda` | 原始 SMART JSON |
| POST | `/storage/smart/test` | `{device,kind:"short"|"long"}` |
| GET | `/storage/smart/tests?device=` | 历史测试 |
| GET | `/storage/pools` | RAID 池 + 实时状态 |
| POST | `/storage/pools` | `{name,level,fs,devices,mountpoint,confirm:"CREATE"}` |
| POST | `/storage/pools/:id/mount` / `unmount` | 挂载/卸载 |
| POST | `/storage/pools/:id/extend` | `{devices:[]}` |
| DELETE | `/storage/pools/:id` | body `{confirm:"DELETE",wipe:bool}` |
| GET | `/storage/mounts` | 挂载点用量 |

## 9. 网络与共享

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/network/interfaces` | 网卡 + 已保存配置 |
| POST | `/network/interfaces/:name` | `{mode,address,gateway,dns,confirm:"APPLY"}` |
| GET | `/network/ports` | 监听端口与进程 |
| GET | `/network/shares` | SMB/NFS 共享、配置路径 |
| POST | `/network/shares` | 新建共享并自动应用 |
| PATCH | `/network/shares/:id` | 修改并应用 |
| DELETE | `/network/shares/:id` | 删除并应用 |
| POST | `/network/apply` | 重新渲染 smb.conf/exports 并 reload |
| GET | `/network/ddns` | DDNS 配置（Token 脱敏） |
| PUT | `/network/ddns` | 保存配置（`__keep__` 保留密钥） |
| POST | `/network/ddns/update` | 立即更新 |
| GET/PUT | `/network/dns` | DNS 服务器列表，PUT `{servers:[]}` |

## 10. 应用中心（管理员）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/apps` | 内置应用目录 |
| POST | `/apps/:slug/install` | 写入 compose 并部署 |
| POST | `/apps/:slug/uninstall` | down + 删除项目，保留数据目录 |
| POST | `/apps/:slug/start` / `stop` | 启停 |
| GET | `/apps/:slug/status` | 容器计数、端口、数据目录 |

## 11. 系统监控

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/monitor/overview` | 当前快照 |
| GET | `/monitor/stream` | SSE，每 2 秒 `event: metrics` |
| GET | `/monitor/history?limit=` | 内存中的历史样本（最多 300） |
| GET | `/monitor/processes` | 进程列表（按 CPU 排序，最多 200） |
| POST | `/monitor/processes/:pid/kill` | `{force:bool}`，管理员 |

## 12. 日志与审计（管理员）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/logs/audit?category=&user=&action=&from=&to=&page=` | 审计日志分页 |
| GET | `/logs/audit/export` | CSV（UTF-8 BOM） |
| GET | `/logs/login?limit=` | 登录尝试 |
| GET | `/logs/system` | 系统事件 + journal 尾部 |
| GET | `/logs/destroy` | 销毁事件（仅专业版有数据） |

## 13. 用户管理（管理员）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/users?query=` | 用户列表（专业版会隐藏他人配额） |
| GET | `/users/deleted` | 冷静期用户（专业版） |
| POST | `/users` | `{username,password,display_name,role,quota_bytes,permissions}` |
| GET | `/users/:id` | 详情 |
| PATCH | `/users/:id` | 修改资料/角色/状态/配额/权限 |
| DELETE | `/users/:id` | 普通版立即删除，专业版进入冷静期 |
| POST | `/users/:id/restore` | 恢复冷静期用户 |
| POST | `/users/:id/password` | 管理员重置密码 |
| POST | `/users/:id/unlock` | 解除锁定、清空失败计数 |
| GET | `/users/:id/usage` | 配额用量（专业版禁止查看他人） |

## 14. 设置与系统（管理员）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/settings` | 全部设置（密钥脱敏）+ 路径 + 版控 |
| PUT | `/settings/:key` | key ∈ security / destroy / deletion / tmdb / notifications / network / ddns / system / normal / https / backup |
| GET | `/system/info` | 硬件、系统、加密、可选组件能力 |
| GET | `/system/updates` | apt 可更新列表 |
| POST | `/system/updates/apply` | 升级（Job） |
| GET | `/system/backup` | 下载配置备份（tar.gz） |
| POST | `/system/restore` | multipart `file` + `confirm=RESTORE` |
| GET | `/system/ssl` | 证书状态 |
| POST | `/system/ssl/selfsigned` | `{domain}` 生成自签名证书 |
| GET | `/system/firewall` | 当前 nftables + 默认规则 |
| PUT | `/system/firewall` | `{ruleset?,extra_ports?,confirm:"APPLY"}`，120 秒内未确认自动回滚 |
| POST | `/system/firewall/confirm` | 保留规则 |
| GET | `/system/encryption` | 根分区加密状态、LUKS 设备、vault 数 |
| GET | `/system/vaults` | 每用户加密卷状态 |
| POST | `/system/vaults/:username/create` | `{size_gb}`，创建并迁移现有文件 |
| POST | `/system/vaults/:username/unlock` / `lock` | 解锁/锁定 |
| GET/PUT | `/system/destroy/policy` | 销毁策略 |
| POST | `/system/destroy` | `{username,confirm:<username>}` 手动销毁（Job） |

### 设置合并语义

`PUT /settings/:key` 支持**部分字段更新**：后端将请求 JSON 深合并到现有配置。
密钥类字段（SMTP 密码、Telegram Token、Bark URL、ServerChan Key、TMDB Key、DDNS Token）
在 GET 中返回 `__keep__`；把 `__keep__` 原样传回即可保留旧值。

## 15. 通知与任务

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/notifications?unread=1&limit=` | 站内通知 + 未读数 |
| POST | `/notifications/read` | `{id}`，0 表示全部已读 |
| DELETE | `/notifications/:id` | 删除通知 |
| POST | `/notifications/test` | 发送测试通知，返回各渠道结果 |
| GET | `/jobs` | 后台任务列表（进度/状态/错误） |
| POST | `/jobs/:id/cancel` | 取消任务 |

## 16. WebDAV

```text
/dav/                      → 当前 Basic Auth 用户的 home
```

- HTTP Basic 认证，账号为 PrivacyOS 用户名/密码。
- 仅允许写入自己的主目录；实现基于 `golang.org/x/net/webdav`。
- 客户端示例：`davfs2`、macOS Finder「连接服务器」、Windows「映射网络驱动器」。
- 建议启用 HTTPS 后再对外使用（Basic Auth 明文传输）。

## 17. SSE 示例

```javascript
const es = new EventSource('/api/v1/monitor/stream?access_token=' + token)
es.addEventListener('metrics', (e) => {
  const snapshot = JSON.parse(e.data)
  console.log(snapshot.cpu, snapshot.mem.percent, snapshot.net.rx_rate)
})
```

## 18. 版本与兼容

- 当前 API 版本：`v1`，随 PrivacyOS 1.0 发布。
- 字段只增不删；新增字段保持向后兼容。
- 破坏性变更会通过 `/api/v2` 提供。
