# PrivacyOS Installer

基于 Debian 13.7 (Trixie) DVD-1 的可引导安装介质的源码。

## 目录

```
preseed/
  common.cfg        公共 preseed（语言、网络、账号、软件包、分区默认值）
  normal.cfg        普通版覆盖项 + late_command
  pro.cfg           专业版 LUKS 覆盖项 + late_command
boot/
  isolinux/         BIOS 菜单（isolinux.cfg/menu.cfg/txt.cfg/gtk.cfg/stdmenu.cfg）
  grub/             UEFI 菜单（grub.cfg、x86_64-efi/grub.cfg）
payload/            安装到目标机 /opt/privacyos 的文件
  install.sh        部署脚本（chroot 内执行，只 enable 不 start）
  banner.sh         开机 banner（名称/IP/版本/版控/官网）
  privacyos-provision.sh  联网补齐 Samba/NFS/Docker
  privacyos-smart-scan.sh 每周 SMART 短测试
  privacyos*.service / .timer
  smb.conf
scripts/
  build-payload.sh  交叉编译后端 + 构建前端 + 组装载荷
  build-iso.sh      解包 Debian ISO、替换菜单/preseed、重新打包
  verify-iso.sh     校验引导记录/preseed/载荷/哈希
  build-in-docker.sh  macOS/Windows 下用 Debian 13 容器构建
Dockerfile.builder   ISO 打包容器
```

## 快速使用

```bash
# 在 Linux（或已安装 xorriso 的环境）上
./scripts/build-payload.sh
./scripts/build-iso.sh --source /path/debian-13.7.0-amd64-DVD-1.iso

# 校验
./scripts/verify-iso.sh ../build/privacyos-1.2.0-amd64.iso
```

完整说明见 [docs/installation.md](../docs/installation.md)。

## 约定

- 启动菜单使用英文（isolinux 对 CJK 字体支持有限），系统内 banner/面板为中文。
- preseed 不支持 `include`，`common.cfg` 与版本文件由构建脚本合并；源文件中的
  `__ADMIN_PASSWORD__` 与 `__LUKS_PASSPHRASE__` 在构建时替换并校验。
- 安装脚本必须在 chroot 中可重复执行，且不得 `systemctl start`。
- 初始管理员密码随机生成，写入目标机 `/root/privacyos-initial-password`（0600）。
- 专业版 LUKS 口令只保存在构建机 `build/LUKS-PASSPHRASE.txt`，不写入安装后的磁盘。
- 提供的 DVD-1 不含 Samba/NFS/Docker 服务端；`privacyos-provision.service` 在联网后补装。
