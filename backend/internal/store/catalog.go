package store

// catalogApps is the built-in application-center catalog. Compose files are
// rendered into /data/docker/apps/<slug>/ and deployed with `docker compose`.
var catalogApps = []App{
	{
		Slug: "qbittorrent", Name: "qBittorrent", Category: "download", Icon: "download",
		Description: "轻量级 BitTorrent 下载器，支持 Web 管理界面与 RSS。",
		Image:       "lscr.io/linuxserver/qbittorrent:latest",
		Ports:       []string{"8081:8081"},
		Compose: `services:
  qbittorrent:
    image: lscr.io/linuxserver/qbittorrent:latest
    container_name: privacyos-qbittorrent
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Asia/Shanghai
      - WEBUI_PORT=8081
    volumes:
      - ./config:/config
      - /data/users:/downloads
    ports:
      - "8081:8081"
    restart: unless-stopped
`,
	},
	{
		Slug: "jellyfin", Name: "Jellyfin", Category: "media", Icon: "film",
		Description: "开源媒体服务器，可作为 PrivacyOS 影视中心的进阶播放后端。",
		Image:       "jellyfin/jellyfin:latest",
		Ports:       []string{"8096:8096"},
		Compose: `services:
  jellyfin:
    image: jellyfin/jellyfin:latest
    container_name: privacyos-jellyfin
    environment:
      - TZ=Asia/Shanghai
      - PUID=1000
      - PGID=1000
    volumes:
      - ./config:/config
      - ./cache:/cache
      - /data/movies:/media
    ports:
      - "8096:8096"
    restart: unless-stopped
`,
	},
	{
		Slug: "nextcloud", Name: "Nextcloud", Category: "tools", Icon: "cloud",
		Description: "私有云盘与协作套件，可与 PrivacyOS 文件目录共存。",
		Image:       "nextcloud:latest",
		Ports:       []string{"8082:80"},
		Compose: `services:
  nextcloud:
    image: nextcloud:latest
    container_name: privacyos-nextcloud
    environment:
      - TZ=Asia/Shanghai
    volumes:
      - ./data:/var/www/html
      - /data/users:/data/users:ro
    ports:
      - "8082:80"
    restart: unless-stopped
`,
	},
	{
		Slug: "adguardhome", Name: "AdGuard Home", Category: "security", Icon: "shield",
		Description: "全网广告拦截与 DNS 隐私保护。",
		Image:       "adguard/adguardhome:latest",
		Ports:       []string{"3000:3000", "53:53/tcp", "53:53/udp"},
		Compose: `services:
  adguardhome:
    image: adguard/adguardhome:latest
    container_name: privacyos-adguardhome
    volumes:
      - ./work:/opt/adguardhome/work
      - ./conf:/opt/adguardhome/conf
    ports:
      - "3000:3000"
      - "53:53/tcp"
      - "53:53/udp"
    restart: unless-stopped
`,
	},
	{
		Slug: "alist", Name: "Alist", Category: "tools", Icon: "folder",
		Description: "多存储聚合文件列表，支持挂载网盘与对象存储。",
		Image:       "xhofe/alist:latest",
		Ports:       []string{"5244:5244"},
		Compose: `services:
  alist:
    image: xhofe/alist:latest
    container_name: privacyos-alist
    volumes:
      - ./data:/opt/alist/data
    ports:
      - "5244:5244"
    restart: unless-stopped
`,
	},
	{
		Slug: "syncthing", Name: "Syncthing", Category: "tools", Icon: "sync",
		Description: "去中心化文件同步，手机与 NAS 之间点对点加密传输。",
		Image:       "syncthing/syncthing:latest",
		Ports:       []string{"8384:8384", "22000:22000/tcp", "22000:22000/udp"},
		Compose: `services:
  syncthing:
    image: syncthing/syncthing:latest
    container_name: privacyos-syncthing
    volumes:
      - ./data:/var/syncthing
      - /data/users:/data/users
    ports:
      - "8384:8384"
      - "22000:22000/tcp"
      - "22000:22000/udp"
    restart: unless-stopped
`,
	},
	{
		Slug: "uptime-kuma", Name: "Uptime Kuma", Category: "tools", Icon: "activity",
		Description: "自托管可用性监控与告警面板。",
		Image:       "louislam/uptime-kuma:1",
		Ports:       []string{"3001:3001"},
		Compose: `services:
  uptime-kuma:
    image: louislam/uptime-kuma:1
    container_name: privacyos-uptime-kuma
    volumes:
      - ./data:/app/data
    ports:
      - "3001:3001"
    restart: unless-stopped
`,
	},
}
