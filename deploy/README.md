# deploy/

预留的部署扩展目录。当前版本的 systemd 单元、Samba 配置与安装脚本位于
`installer/payload/`，由 `build-payload.sh` 打包进 ISO 并在 `install.sh` 中部署。

如需增加：

- Kubernetes/Helm：将 `privacyos-server` 作为无状态 API + 挂载 `/data`；
- 反向代理：Nginx/Caddy 配置（面板本身也可直接提供 TLS）；
- 监控：Prometheus node_exporter 与 PrivacyOS `/api/v1/monitor/overview` 并存；
- 备份：restic/borg 对 `/etc/privacyos` 与 `/data/users` 做异地备份。

请保持与安装器使用相同的目录约定（`/opt/privacyos`、`/etc/privacyos`、`/data`）。
