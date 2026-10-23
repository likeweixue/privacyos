# 安装与 ISO 构建

本文说明如何把官方 `debian-13.7.0-amd64-DVD-1.iso` 重新打包为 **PrivacyOS 安装 ISO**，
以及安装完成后的首次启动行为。

## 1. 构建环境

| 项目 | 要求 |
| --- | --- |
| 操作系统 | **Linux**（Debian 13 / Ubuntu 24.04 推荐）。macOS/Windows 请用 Docker 或 CI |
| 工具 | `xorriso`、`isolinux`/`syslinux-common`、`mtools`、`dosfstools`、`sha256sum` |
| 源 ISO | `debian-13.7.0-amd64-DVD-1.iso`（约 4 GB） |
| 构建载荷 | Go 1.24+、Node.js 20+（仅构建机需要，目标机不需要） |

Debian/Ubuntu 安装依赖：

```bash
sudo apt-get update
sudo apt-get install -y xorriso isolinux syslinux-common mtools dosfstools \
  ca-certificates curl file
```

> macOS 上 `hdiutil` 可以挂载 ISO，但无法生成 Debian 所需的 BIOS+UEFI 混合引导记录，
> 因此 `build-iso.sh` 在 Darwin 上会直接退出并给出 Docker 方案。

## 2. 构建载荷

```bash
cd privacyos
./installer/scripts/build-payload.sh
```

脚本会：

1. `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build` 生成 `build/privacyos-server`；
2. `npm run build` 生成前端 `frontend/dist`；
3. 复制 `installer/payload` 到 `build/payload/`；
4. 生成初始管理员密码，写入：
   - `build/payload/initial-admin-password`（随 ISO 进入安装器）
   - `build/initial-admin-password`（构建机保存，0600）

可自定义：

```bash
PRIVACYOS_ADMIN_PASSWORD='MyStrongInitialPassw0rd' \
  ./installer/scripts/build-payload.sh
```

密码仅允许字母数字，避免 preseed 转义问题，长度至少 12 位。

载荷结构：

```text
build/payload/
├── privacyos-server             linux/amd64 静态二进制
├── web/                         前端静态文件
├── install.sh                   安装脚本（preseed late_command 调用）
├── banner.sh                    开机 banner
├── privacyos-provision.sh       可选组件补装
├── privacyos-smart-scan.sh      每周 SMART 短测试
├── privacyos.service
├── privacyos-banner.service
├── privacyos-provision.service
├── privacyos-smart.service / .timer
├── smb.conf
├── version
└── initial-admin-password       安装后由 install.sh shred
```

## 3. 构建 ISO

```bash
./installer/scripts/build-iso.sh \
  --source "../开发文档/debian-13.7.0-amd64-DVD-1.iso" \
  --output "build/privacyos-1.1.0-amd64.iso"
```

脚本流程：

1. **解包**：`xorriso -osirrox on -indev <iso> -extract / build/iso-work/iso`
   （无需 root、无需 loop 挂载）；
2. **替换启动菜单**：`isolinux/*.cfg`、`boot/grub/grub.cfg`、`x86_64-efi/grub.cfg`；
3. **合并 preseed**：`common.cfg + normal.cfg` → `/preseed/normal.cfg`，
   `common.cfg + pro.cfg` → `/preseed/pro.cfg`，并替换：
   - `__ADMIN_PASSWORD__` → 载荷中的初始密码
   - `__LUKS_PASSPHRASE__` → 构建时随机生成的 28 位口令
4. **写入载荷**：`/privacyos/*`；
5. **品牌信息**：`/.disk/info`、`/privacyos/INSTALL.txt`；
6. **md5sum**：重新生成 `md5sum.txt`（`--no-md5` 可跳过，节省几分钟）；
7. **打包**：xorriso `-as mkisofs`，保留 BIOS `isolinux.bin` 与 UEFI `efi.img`，
   生成 GPT/ISO 混合镜像；
8. **校验**：El Torito 记录、大小、SHA256。

可选参数：

| 参数 | 说明 |
| --- | --- |
| `--source` | 源 Debian ISO |
| `--payload` | 载荷目录（默认 `build/payload`） |
| `--output` | 输出 ISO |
| `--work` | 解包工作目录（默认 `build/iso-work`） |
| `--no-md5` | 跳过 md5sum 生成 |
| `--keep` | 保留解包目录，便于二次构建 |
| `PRIVACYOS_LUKS_PASSPHRASE` | 指定专业版 LUKS 口令（默认随机） |

产物：

```text
build/privacyos-1.1.0-amd64.iso
build/privacyos-1.1.0-amd64.iso.sha256
build/LUKS-PASSPHRASE.txt      # 0600
```

## 4. 在 Docker 中构建（macOS / Windows）

```bash
# 1) 先在宿主机构建载荷（需要 Go + Node）
./installer/scripts/build-payload.sh

# 2) 使用 Debian 13 容器重新打包
./installer/scripts/build-in-docker.sh \
  --source "/path/to/debian-13.7.0-amd64-DVD-1.iso" \
  --output "$PWD/build/privacyos-1.1.0-amd64.iso"
```

容器镜像 `installer/Dockerfile.builder` 只包含 `xorriso`/`isolinux` 等打包工具，
不会下载或构建 Go/Node 依赖。

## 5. 校验 ISO

```bash
./installer/scripts/verify-iso.sh build/privacyos-1.1.0-amd64.iso
```

检查项：

- BIOS（`isolinux.bin`）与 UEFI（`efi.img`）双引导记录；
- `/preseed/normal.cfg`、`/preseed/pro.cfg` 存在且无残留占位符；
- 专业版 preseed 含 `partman-auto/method crypto`；
- 启动菜单引用专业版 preseed；
- `/privacyos/privacyos-server` 为 linux/amd64 ELF；
- SHA256 匹配（存在 `.sha256` 时）。

## 6. 写入 U 盘并安装

```bash
# Linux / macOS（确认盘符后执行！）
sudo dd if=build/privacyos-1.1.0-amd64.iso of=/dev/rdiskX bs=4m status=progress && sync
```

Windows 可使用 Rufus（DD 模式）或 balenaEtcher。

启动菜单（英文以避免 isolinux 中文乱码）：

```text
Install PrivacyOS (Normal edition)                        ← 默认
Install PrivacyOS (Pro edition - full disk encryption)
Install PrivacyOS (Normal edition - graphical)
Install PrivacyOS (Pro edition - graphical)
Advanced options ... → Manual install / Rescue mode / Boot from hard disk
```

安装过程（preseed 自动应答）：

1. 语言 `zh_CN.UTF-8`、时区 `Asia/Shanghai`；
2. 自动 DHCP；主机名 `privacyos`；
3. 普通版：单分区 `atomic`；专业版：LUKS2 全盘加密 + LVM；
4. 创建 `padmin` 账号（密码为构建时生成的初始密码）；
5. 从 DVD 安装核心软件包；
6. `late_command` 把 `/cdrom/privacyos` 复制到 `/target/opt/privacyos`，
   写入版本标记，执行 `install.sh`；
7. 重启后进入系统，控制台显示 PrivacyOS banner（名称、IP、版本、Web 地址、官网）。

安装后凭据：

| 项目 | 位置 |
| --- | --- |
| Web 管理员 | `padmin` / `/root/privacyos-initial-password` |
| 专业版 LUKS 口令 | 构建机 `build/LUKS-PASSPHRASE.txt`（开机解密必需） |
| 数据目录 | `/data` |
| 配置目录 | `/etc/privacyos` |

> **专业版注意**：全盘加密意味着每次开机都要输入 LUKS 口令；NAS 无人值守场景需自行评估。
> 自动化安装使用的是构建时随机口令，请尽快登录后执行：
>
> ```bash
> sudo cryptsetup luksChangeKey /dev/nvme0n1p3   # 替换为实际设备
> ```

## 7. 首次启动与可选组件

`privacyos.service` 立即启动面板；`privacyos-provision.service` 在联网时并行执行：

1. 写入 `deb.debian.org` 在线源；
2. 安装 `samba`、`nfs-kernel-server`、`docker.io`、`docker-compose`、
   `libimage-exiftool-perl`、`nvme-cli`、`iotop`、`iftop`、`quota` 等；
3. 为 `padmin` 创建 Samba 账户；
4. 启动 Samba/NFS/Docker；
5. 写入 `/var/lib/privacyos/provisioned` 标记。

**提供的 DVD-1 不包含上述服务端包**，因此：

- 有网络：首次启动几分钟后功能自动就绪；
- 无网络：面板正常使用文件/相册/影视/存储/监控，Samba/NFS/Docker 显示未安装；
  联网后执行 `sudo privacyos-provision --force` 即可补装。

系统信息页会显示各组件能力（samba/nfs/docker/ffmpeg/smartctl/cryptsetup/mdadm/nft）。

## 8. 安装器细节

### preseed 合并

d-i 的 preseed 不支持 `include`，所以仓库中把公共项与版本项拆开，
由 `build-iso.sh` 合并后再写入 ISO：

```bash
cat preseed/common.cfg preseed/normal.cfg > /preseed/normal.cfg
sed -i 's/__ADMIN_PASSWORD__/.../g; s/__LUKS_PASSPHRASE__/.../g' /preseed/normal.cfg
```

### 为什么不使用固定密码

需求文档中的示例密码（`privacyos` / `privacyos-luks`）会被任何拿到 ISO 的人知晓。
本项目改为**构建时随机生成**，并分别保存在构建机与目标机 0600 文件中。
如需完全交互式设置，可在启动菜单选择 `Manual install` 手动安装 Debian，
再执行 `install.sh --start`。

### 分区

- 普通版：`atomic`（单分区 ext4），适合快速体验；数据盘可在「存储管理」中追加。
- 专业版：`crypto` + `atomic` + LVM，LUKS2 全盘加密。
- 若需要 RAID 根分区，请使用 `Manual install` 后运行安装脚本。

## 9. 自定义

| 需求 | 修改位置 |
| --- | --- |
| 默认软件包 | `installer/preseed/common.cfg` 的 `pkgsel/include` |
| 时区/语言/主机名 | `common.cfg` |
| 默认分区 | `normal.cfg` / `pro.cfg` |
| 启动菜单文字 | `installer/boot/isolinux/*.cfg`、`boot/grub/grub.cfg` |
| 开机 banner | `installer/payload/banner.sh` |
| systemd 加固 | `installer/payload/privacyos.service` |
| Samba 默认配置 | `installer/payload/smb.conf`（面板保存后会覆盖） |

修改后重新执行 `build-payload.sh` 与 `build-iso.sh`。

## 10. 故障排查

| 现象 | 排查 |
| --- | --- |
| `xorriso` 不存在 | 安装 `xorriso isolinux syslinux-common` |
| 找不到 `isohdpfx.bin` | 安装 `isolinux` 或 `syslinux-common` |
| ISO 只有几百 MB | 解包/复制失败；检查源 ISO 与磁盘空间（需 ≥ 12 GB） |
| 启动黑屏/UEFI 无菜单 | 确认使用了 `verify-iso.sh`；尝试关闭 Secure Boot 或切 BIOS 模式 |
| 安装停在分区确认 | preseed 未加载：检查菜单 `preseed/file=` 路径与 `/preseed/*.cfg` 是否存在 |
| 安装后无面板 | `systemctl status privacyos`、`journalctl -u privacyos -n 100` |
| Samba/NFS/Docker 缺失 | 联网后 `sudo privacyos-provision --force` 或手动 `apt install` |
| 专业版开机会话卡住 | 需要输入 LUKS 口令；见 `build/LUKS-PASSPHRASE.txt` |
| 忘记 Web 密码 | 停止服务，删除数据库后重启会重新种子（会丢失用户配置），或使用 `padmin` 的初始密码文件 |

## 11. 支持矩阵

| 场景 | 支持 |
| --- | --- |
| BIOS 传统启动 | ✅ isolinux |
| UEFI 启动 | ✅ GRUB（沿用 Debian shim/efi.img） |
| Secure Boot | ✅ 未替换签名二进制（自定义 grub.cfg 在签名后加载） |
| 离线安装核心系统 | ✅ 仅使用 DVD-1 内软件包 |
| 离线安装 Samba/NFS/Docker | ❌ 需联网补装 |
| 服务器无显示器 | ✅ 安装后通过 IP 访问；LUKS 专业版需串口/IPMI 输入口令 |
