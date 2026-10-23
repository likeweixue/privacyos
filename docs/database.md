# 数据库设计（SQLite）

PrivacyOS 使用嵌入式 SQLite（`modernc.org/sqlite`，纯 Go、无 CGO），数据库文件位于
`/etc/privacyos/privacyos.db`。WAL 模式、外键开启、30 秒写锁超时。

- Schema 版本记录在 `PRAGMA user_version`；
- 迁移文件位于 `backend/internal/store/migrations/*.sql`，按文件名前缀顺序执行；
- 时间统一存储为 **RFC3339 UTC 字符串**（可字典序排序）；
- 布尔值使用 INTEGER 0/1；
- JSON 字段（权限、偏好、设备列表、端口、标签）以 TEXT 存储。

## 1. 关系概览

```text
users 1─────n sessions
  │ 1─────n shares
  │ 1─────n trash_items
  │ 1─────n photos 1─────n album_photos n─────1 albums
  │ 1─────n movies
  └─ 1──n login_attempts（按 username，保留历史）

settings / audit_logs / notifications / alert_events   独立表
storage_pools / smart_tests                            独立表
apps / network_shares / destroy_events                 独立表
```

## 2. 表结构

### users

| 列 | 类型 | 说明 |
| --- | --- | --- |
| id | INTEGER PK | 自增 |
| username | TEXT UNIQUE | 同时作为 Linux/Samba 用户名 |
| display_name | TEXT | 显示名 |
| password_hash | TEXT | bcrypt(cost 12) |
| role | TEXT | `admin` / `user` |
| status | TEXT | `active` / `disabled` / `pending_deletion` |
| quota_bytes | INTEGER | 0 = 不限 |
| permissions | TEXT JSON | files/photos/movies/share/docker/apps |
| preferences | TEXT JSON | `{lang,theme}` |
| totp_secret / totp_enabled | TEXT / INTEGER | TOTP 2FA |
| locked_until | TEXT | 临时锁定截止时间 |
| last_login_at / last_login_ip | TEXT | 最近登录 |
| delete_after | TEXT | 冷静期到期时间 |
| created_at / updated_at | TEXT | |

索引：`status`。

### sessions

| 列 | 说明 |
| --- | --- |
| id | UUID，即 JWT 的 `jti` |
| user_id | 外键 → users，级联删除 |
| ip / user_agent | 登录上下文 |
| created_at / last_seen / expires_at | 生命周期 |
| revoked_at | 非空即已注销 |

索引：`user_id`。过期会话由 janitor 与登录流程清理。

### login_attempts

`id, username, ip, success, reason, created_at`。用于计算失败次数与退避，
按 `(username, created_at)` 建索引；保留 90 天。

### audit_logs

`id, created_at, user_id, username, category, action, target, detail, ip, result`。

- category：`auth / file / user / photo / movie / docker / storage / network / app / system / destroy`
- 索引：`created_at`、`(category, created_at)`、`(username, created_at)`
- 默认保留 90 天，可在设置中调整清理策略（janitor 定期清理）。

### settings

`key TEXT PRIMARY KEY, value TEXT(JSON), updated_at`。当前键：

`security`、`destroy`、`deletion`、`tmdb`、`notifications`、`network`、`ddns`、
`system`、`normal`、`https`、`backup`。

设置采用 JSON 深合并更新，敏感字段在 API 返回时替换为 `__keep__`。

### shares

| 列 | 说明 |
| --- | --- |
| id | UUID |
| token | UNIQUE，URL 安全随机 token（18 字节 hex） |
| name / path | 展示名 / 绝对路径（仅在服务端使用） |
| owner_id | 外键 → users |
| kind | `file` / `dir` |
| password_hash | 可空，bcrypt |
| expires_at | 可空 |
| download_only | 默认 1 |
| max_downloads / downloads | 次数限制与计数 |
| size_bytes | 元数据 |
| created_at / last_access_at | |

### trash_items

`id, owner_id, original_path, trash_path, name, is_dir, size_bytes, deleted_at, expires_at`。
到期后由 janitor `os.RemoveAll` 并删除记录。

### storage_pools

`id, name UNIQUE, level(raid0/1/5/6/10/linear), fs, devices(JSON), mountpoint,
status, size_bytes, created_at, updated_at`。

### smart_tests

`id, device, kind(short/long), status, summary, raw(JSON), created_at`。保留最近 20 条供 UI 展示。

### photos

| 列 | 说明 |
| --- | --- |
| id | INTEGER PK |
| owner_id | → users |
| path | 绝对路径，**不通过 API 暴露** |
| file_name / size_bytes / width / height | |
| taken_at | EXIF 拍摄时间，缺失时用文件 mtime |
| camera / lens | EXIF |
| gps_lat / gps_lng | 可空，地图视图 |
| orientation | 1–8 |
| thumb_path / sha256 | 缓存与去重 |
| created_at | |

唯一约束 `(owner_id, path)`；索引 `(owner_id, taken_at)`。

### albums / album_photos

`albums(id, owner_id, name, description, cover_photo, created_at)`，
`UNIQUE(owner_id, name)`；`album_photos(album_id, photo_id, added_at)` 复合主键。

### movies

| 列 | 说明 |
| --- | --- |
| id / owner_id | |
| path / file_name / size_bytes | 路径仅服务端使用 |
| title / original_title / year | 文件名推断或 TMDB |
| kind | movie / series / anime / documentary |
| season / episode | 剧集 |
| overview / poster / backdrop | 刮削结果（poster 为本地缓存路径） |
| rating / runtime_seconds / genres / cast_list | |
| tmdb_id / resolution / video_codec / audio_codec | |
| scraped | 0/1 |
| created_at | |

唯一约束 `(owner_id, path)`。

### apps

`slug PK, name, category, description, icon, image, ports(JSON), compose(TEXT),
installed, status, installed_at, updated_at`。内置目录首次启动时 upsert，
`installed/status` 保留用户状态。

### network_shares

`id, name UNIQUE, path, comment, protocols(JSON: smb/nfs), read_only,
valid_users, guest_ok, created_at`。面板修改后重新渲染 Samba/NFS 配置。

### destroy_events

`id, created_at, username, trigger, method, status(running/completed/failed), detail`。
一次销毁只产生一条记录：先插入 `running`，完成后更新为 `completed/failed`。

### notifications

`id, level(info/warning/critical), title, message, read, created_at`。
UI 顶栏铃铛、未读数与设置页测试通知共用。

### alert_events

`id, key, level, title, message, created_at, resolved_at`。用于去重的系统告警
（磁盘 SMART、默认密码等）；同一 key 在未解决前不会重复插入。

## 3. 数据保留

| 数据 | 保留策略 |
| --- | --- |
| sessions | 过期后清理（每 30 分钟） |
| login_attempts | 90 天 |
| notifications | 90 天 |
| audit_logs | 90 天（保留策略可调整） |
| trash | 默认 30 天，可在设置中修改 |
| pending_deletion 用户 | 冷静期（默认 30 天）结束后永久删除数据与记录 |
| smart_tests | 每设备最近 20 条（查询限制） |

## 4. 备份与恢复

- `GET /system/backup` 打包 `/etc/privacyos`（含数据库、密钥、设置），**不含媒体文件**。
- 备份前会生成当前配置副本；恢复时要求 `confirm=RESTORE`，恢复后需重启服务。
- 建议同时定期备份 `/data/users`（rsync/SMB），或使用存储池快照。

## 5. 并发与性能

- 连接池最多 8 个连接；WAL 允许读写并发。
- 高频写：`sessions.last_seen`（5 分钟节流）、审计日志、登录尝试。
- 大目录列表最多返回 5000 项并标记 `truncated`。
- 相册时间轴在数据库内按日聚合，前端只渲染当前页。
- 配额用量通过文件系统遍历计算，带 30 秒内存缓存；上传/删除时失效。

## 6. 迁移指南

新增迁移：

1. 新建 `backend/internal/store/migrations/0002_xxx.sql`；
2. 文件开头编号必须大于当前 `user_version`；
3. 迁移在事务中执行（SQLite 部分 DDL 不支持事务内的复杂变更，需拆分为多条语句）；
4. 启动时自动应用并更新 `user_version`；
5. 同步更新本文档与 `store` 中的扫描字段。

> 升级前请先备份 `/etc/privacyos/privacyos.db` 与 `privacyos.env`。
