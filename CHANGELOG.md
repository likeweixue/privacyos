# Changelog

## 1.1.0 — 2026-10-24

管理界面重构：从 Windows 风格桌面壳切换为 Flarum 2.0 Admin 风格的平面后台 SPA。

### 变更

- 移除桌面窗口、任务栏、开始菜单，改为固定顶栏 + 侧边导航 + 内容区
- 区域配色：顶栏/侧边栏为白色，内容画布为浅灰，Widget 为白色线框卡片
- 全局扁平化：去除大圆角、悬浮阴影与岛屿式卡片，统一 1–3px 圆角和 1px 边框
- 默认浅色主题，保留深色主题切换
- 页面标题收进标准 AdminHeader，文件管理与设置页改为连续区域布局

### 新增

- `App-header` 固定顶栏：返回、品牌、搜索、帮助、会话下拉
- `App-nav` 固定侧边栏 + 移动端 Drawer
- 可搜索的 `Dropdown--select` 导航（渐进式披露全部模块）
- 命令面板（⌘/Ctrl+K），键盘上下选择、回车打开
- Dashboard Widget 流：StatusWidget / StatisticsWidget / AnnouncementsWidget / ExtensionsWidget
- 状态点、工具下拉（清除缓存/系统信息/高级设置）、公告与扩展列表
- a11y：skip link、aria-label、aria-current、role=search/listbox/menu
- `scripts/start-demo.sh` / `stop-demo.sh`：后台运行完整演示服务

## 1.0.0 — 2026-10-24

首个完整版本。

### 新增

- 基于 Debian 13.7 Trixie 的 preseed 安装器（普通版 / 专业版）
- 自定义 isolinux + GRUB 启动菜单，BIOS + UEFI 双引导
- Go 后端：认证、会话、2FA、用户/权限、文件沙箱、回收站、分享、配额
- 相册：EXIF 归档、时间轴、缩略图、GPS 视图、手动相册
- 影视中心：媒体库扫描、ffprobe、TMDB 刮削、Range 播放、字幕转换
- Docker：容器/镜像/Compose 管理与日志
- 存储：磁盘 SMART、mdadm 存储池、挂载点用量
- 网络：网卡配置、SMB/NFS 导出、端口、DDNS、DNS、nftables
- 应用中心：7 个内置应用模板
- 监控：SSE 实时指标、进程管理、温度
- 审计：登录/操作/系统/销毁日志、CSV 导出
- 设置：更新、HTTPS、通知、备份恢复、安全策略
- 专业版：LUKS 全盘加密安装、每用户加密卷、销毁策略、30 天冷静期
- Vue 3 + Element Plus 前端，深色/浅色主题，中英文界面
- Windows 风格桌面壳：任务栏、开始菜单、多窗口（1.1.0 中替换为后台 SPA）
- 单元测试、CI、Docker 开发环境、完整文档

### 已知问题

- 提供的 DVD-1 不含 Samba/NFS/Docker 服务端，需联网补装
- 人脸识别、实时转码、地图底图未实现
- 尚未在真实 Debian 硬件上完成 ISO 端到端安装验证
