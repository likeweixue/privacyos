# 开发指南

## 1. 环境

| 工具 | 版本 | 用途 |
| --- | --- | --- |
| Go | 1.24+（go.mod 指令 1.23） | 后端、交叉编译 |
| Node.js | 20+（推荐 22/24） | 前端构建 |
| npm | 10+ | 依赖管理 |
| Debian 13 / Docker | 可选 | 构建 ISO、真实环境验证 |

本地开发不需要 Debian：`PRIVACYOS_DEV=1`（非 Linux 自动开启）会启用全部模拟层。

## 2. 启动

```bash
make dev
# 或
./scripts/dev.sh
```

- 后端 `127.0.0.1:8080`，前端 `127.0.0.1:5173`（Vite 代理 `/api`）。
- 数据：`dev-data/{data,etc,log}`；删除该目录即可重置演示数据。
- 默认账号：`padmin / privacyos`，`demo / demo1234`。
- 演示数据包含 12 张生成的示例照片、6 部电影条目、若干文件/审计/通知；
  `PRIVACYOS_DEMO=0` 可关闭。

单独启动：

```bash
cd backend && PRIVACYOS_DEV=1 go run ./cmd/privacyos-server
cd frontend && npm run dev
```

常用环境变量：

| 变量 | 默认（开发） | 说明 |
| --- | --- | --- |
| `PRIVACYOS_DEV` | 非 Linux 自动为 1 | 模拟硬件、演示数据 |
| `PRIVACYOS_LISTEN` | `127.0.0.1:8080` | 监听地址 |
| `PRIVACYOS_DATA_DIR` | `./dev-data/data` | 数据根 |
| `PRIVACYOS_CONFIG_DIR` | `./dev-data/etc` | 配置与数据库 |
| `PRIVACYOS_LOG_DIR` | `./dev-data/log` | 日志 |
| `PRIVACYOS_WEB_DIR` | `../frontend/dist` | 静态文件 |
| `PRIVACYOS_EDITION` | `normal` | `normal` / `pro` |
| `PRIVACYOS_ADMIN_PASSWORD` | `privacyos` | 首次种子密码 |
| `PRIVACYOS_DB` | `<config>/privacyos.db` | 数据库路径 |
| `PRIVACYOS_TLS_CERT/KEY` | 空 | 启用 HTTPS |

## 3. 目录结构

```text
backend/
├── cmd/privacyos-server/main.go     入口、banner、DDNS 循环
└── internal/
    ├── api/                         路由、中间件、handlers_*.go
    ├── auth/                        JWT / bcrypt / TOTP
    ├── config/                      路径与版控解析
    ├── core/                        App、policy、vault、jobs、notify、janitor
    ├── dockerx/                     Docker Engine API + Compose
    ├── files/                       文件沙箱、上传、回收站、压缩、缩略图
    ├── media/                       photos.go / movies.go / TMDB
    ├── monitor/                     /proc 采样 + SSE
    ├── shares/                      Samba/NFS 渲染与应用
    ├── storage/                     lsblk / smartctl / mdadm
    ├── store/                       SQLite、migrations、seeds、models
    ├── system/                      主机信息、网络、更新、SSL、防火墙、DDNS
    ├── thumb/                       图片/视频缩略图
    └── version/
frontend/
├── src/api/index.js                 所有 REST 客户端 + token 刷新
├── src/stores/{auth,ui,desktop,layout,tasks}.js  Pinia（认证/主题/窗口/边栏/任务）
├── src/desktop/                    桌面壳：Desktop / WindowFrame / Taskbar / StartMenu
├── src/apps.js                     窗口组件与导航注册表（分组、图标、尺寸）
├── src/views/*.vue                  12 个应用页面 + 登录 + 公开分享 + 404
├── src/components/                  PageHeader/StatCard/EChart/...
├── src/locales/{zh-CN,en-US}.js     词条（运行 scripts/check-i18n.mjs 校验）
└── scripts/check-i18n.mjs
```

## 4. 后端开发约定

### 新增一个 API

1. 在对应 `internal/<module>` 实现业务逻辑，返回 `error`；
2. 在 `internal/api/handlers_*.go` 写 handler：
   - `bindJSON` 校验；
   - 使用 `currentUser(c)` 获取身份；
   - 使用 `requireAdmin` / `feature(...)` / `adminOnly()` 控制权限；
   - 成功 `ok(c, data)` / `created(c, data)`；
   - 失败 `apiError(c, status, code, message)`；
   - 敏感操作调用 `s.audit(...)`。
3. 在 `internal/api/server.go` 注册路由；
4. 在 `frontend/src/api/index.js` 增加客户端方法；
5. 补充测试与文档（`docs/api.md`）。

### 错误与日志

- 对外错误必须使用稳定 `code` + 中文 `message`，不要泄露绝对路径/堆栈。
- 审计写入失败只记录日志，不影响请求。
- 后台任务使用 `core.Jobs`，前端轮询 `/jobs` 展示进度。

### 数据库

- 修改 schema 必须新增迁移文件，不可修改已发布迁移。
- `store` 负责扫描/JSON 编解码；handler 不直接拼 SQL。
- 时间统一 `store.Now()` / `store.Time()`。

### 路径与权限

- 所有用户文件路径必须经过 `files.SecureJoin` / `Service.Resolve`。
- 专业版隔离由 `App.CanSeeOtherFiles` 统一裁决，禁止在 handler 中另写版控判断。
- 需要 root 的系统操作（mdadm/cryptsetup/systemctl）应：
  - 校验参数（设备名、用户名、路径）；
  - 使用 `exec.CommandContext` + 超时；
  - 不拼接 shell 字符串；
  - 失败时返回可读错误并写入审计。

### 开发模式模拟

`config.Dev` 为真时：

- `storage` 返回 4 块模拟磁盘与 SMART；
- `dockerx` 返回 4 个容器/镜像/日志/统计；
- `monitor` 生成正弦波指标与进程列表；
- `system` 返回模拟网络接口、端口、更新、加密状态；
- `core.CreateVault/DestroyVault` 使用标记文件而非真实 cryptsetup；
- `files/service.applyOwner` 在非 root 时跳过 chown。

新增系统调用时请同时提供 dev 分支，保证 `make dev` 在 macOS 可用。

## 5. 前端开发约定

- Vue 3 `<script setup>`，不使用 TypeScript（当前）。
- 所有用户可见文本走 `t('key')`；新增词条同时写 `zh-CN.js` 与 `en-US.js`。
- 主题使用 CSS 变量：`--pos-bg/panel/border/text/muted/accent/danger`，
  组件样式使用 `scoped`。
- 页面结构：`.pos-page` → `<PageHeader>` → 内容面板 `.pos-panel`。
- 错误统一 `ElMessage.error(errorMessage(error))`；危险操作 `ElMessageBox.confirm`。
- 图表统一使用 `components/EChart.vue`（自动 resize，主题感知）。
- 图片/视频 URL 通过 `apiURL()` 附加 token（`<img>`/`<video>` 不能设置 Header）。

### i18n 检查

```bash
cd frontend && node scripts/check-i18n.mjs
```

脚本扫描所有 `t('...')` 字面量并与 `zh-CN.js` 比对；CI 会执行。

### 状态管理

- `auth`：当前用户、版控、功能权限；登录后调用 `/auth/me`。
- `ui`：主题、语言（localStorage 持久化）。
- `desktop`：窗口列表、多实例、焦点层级、最小化/最大化、开始菜单。
- 页面内部状态使用 `ref/reactive`，不引入额外全局 store。

## 6. 测试

```bash
make test          # go test ./...
make check         # go vet + test + i18n + shell 语法
```

现有测试覆盖：

- `files`：路径穿越、符号链接逃逸、上传/删除/恢复、配额、移动/复制、zip；
- `core`：退避算法、锁定/解锁、专业版销毁触发、销毁落盘、vault、janitor；
- `auth`：bcrypt、JWT 签名/过期/篡改、TOTP；
- `media`：文件名解析、SRT→VTT、EXIF 回退；
- `shares`：Samba/NFS 渲染、专业版加固项。

建议新增：

- handler 级 httptest（当前主要依赖 curl/CDP 冒烟）；
- 并发上传/配额竞争测试；
- 真实 Debian 上的安装/升级集成测试。

## 7. 调试

- 后端日志：`tail -f /tmp/privacyos-server.log`（`make dev` 输出到终端）。
- 数据库：`sqlite3 dev-data/etc/privacyos.db '.tables'`。
- 前端：Vite HMR；网络面板查看 `/api` 请求；SSE 在 `monitor` 页。
桌面路由：`/` 为空桌面，`/dashboard`、`/files`、`/settings` 等为深链接；
父路由 `Desktop` 不使用 `<router-view>`，`src/apps.js` 注册表为每个窗口挂载独立组件实例，
因此同一页面可以同时打开多个窗口。
- 无头浏览器冒烟（本项目开发时使用）：Chrome CDP 登录后逐页截图，检查
  `.el-message--error` 与控制台异常。
- 重置数据：`rm -rf dev-data`，重启后重新种子。

### macOS 原生模块签名问题

若 npm 安装后 Vite 报错
`code signature ... different Team IDs`（rollup/esbuild 原生模块），可：

```bash
npm install --no-save @rollup/wasm-node@4
mv node_modules/rollup node_modules/rollup.orig
ln -s @rollup/wasm-node node_modules/rollup
```

或直接在 Linux 容器中构建前端。这是本机 Node 签名环境问题，与项目代码无关。

## 8. 构建与发布

```bash
make all           # 后端 + 前端
make payload       # 组装 ISO 载荷
make iso           # Linux 上生成 ISO
make verify
```

发布检查清单：

- [ ] `make check` 通过
- [ ] `go vet ./...` 无告警
- [ ] 前端构建成功，i18n 无缺失
- [ ] 后端 linux/amd64 静态二进制（`CGO_ENABLED=0`）
- [ ] 更新 `VERSION`
- [ ] 更新 `CHANGELOG.md`
- [ ] 在 Debian 13 实机/VM 执行 ISO 安装
- [ ] 验证普通版/专业版菜单、banner、面板登录、SMB
- [ ] 专业版验证 LUKS 口令、销毁策略、冷静期
- [ ] 备份并归档 `build/LUKS-PASSPHRASE.txt`

## 9. 代码风格

- Go：`gofmt`，错误使用 `fmt.Errorf("...: %w", err)`，避免 panic。
- Shell：`set -Eeuo pipefail`，变量加引号，`bash -n` 校验。
- Vue：单文件组件，`<script setup>` 顺序为 imports → refs → computed →
  functions → lifecycle；样式使用设计令牌。
- 提交信息：`feat: ...`、`fix: ...`、`docs: ...`、`chore: ...`。
