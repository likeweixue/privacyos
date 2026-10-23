package store

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"privacyos/backend/internal/config"
)

// seedDemo creates a demo account plus generated photos, files and audit
// history. It only runs in development mode.
func (s *Store) seedDemo(cfg *config.Config) error {
	if _, err := s.GetUserByUsername("demo"); err == nil {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("demo1234"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u := &User{
		Username: "demo", DisplayName: "演示用户", PasswordHash: string(hash),
		Role: "user", Status: "active", Permissions: DefaultPermissions("user"),
		Preferences: Preferences{Lang: "zh-CN", Theme: "dark"},
	}
	if err := s.CreateUser(u); err != nil {
		return err
	}

	home := filepath.Join(cfg.DataDir, "users", u.Username)
	photoRoot := filepath.Join(cfg.DataDir, "photos", u.Username)
	_ = os.MkdirAll(home, 0o700)
	_ = os.MkdirAll(photoRoot, 0o700)
	_ = os.MkdirAll(filepath.Join(cfg.DataDir, "movies", u.Username), 0o700)

	writeDemoFiles(home)
	if err := s.seedDemoPhotos(cfg, u, photoRoot); err != nil {
		return err
	}
	if err := s.seedDemoMovies(cfg, u); err != nil {
		return err
	}
	s.seedDemoActivity(u)
	return nil
}

func writeDemoFiles(home string) {
	files := map[string]string{
		"欢迎使用 PrivacyOS.txt": "欢迎使用 PrivacyOS\n\n" +
			"1. 左侧文件树用于浏览个人目录。\n" +
			"2. 所有文件都保存在 /data/users/<用户名>。\n" +
			"3. 专业版启用 Linux 权限隔离，管理员也无法读取其他用户文件。\n",
		"Documents/家庭数据备份说明.md": "# 家庭数据备份\n\n- 每周日 03:00 自动备份到 /data/backups\n- 保留最近 4 份快照\n- 备份文件使用 AES-256 加密\n",
		"Documents/隐私清单.txt": "隐私检查清单\n\n[ ] 开启双重认证\n[ ] 修改默认密码\n[ ] 配置 HTTPS\n[ ] 检查共享目录权限\n[ ] 设置磁盘 SMART 告警\n",
		"Projects/roadmap.md": "# PrivacyOS 路线图\n\n- [x] 基础系统与 Web 面板\n- [x] 文件与用户隔离\n- [ ] 相册人脸识别\n- [ ] 影视转码\n",
		"Media/readme.txt": "把影片放入 /data/movies/<用户名> 后，在影视中心点击“扫描媒体库”。\n",
	}
	for rel, body := range files {
		abs := filepath.Join(home, rel)
		_ = os.MkdirAll(filepath.Dir(abs), 0o700)
		_ = os.WriteFile(abs, []byte(body), 0o600)
	}
}

func (s *Store) seedDemoPhotos(cfg *config.Config, u *User, root string) error {
	photos := []struct {
		date   string
		camera string
		lat    float64
		lng    float64
		title  string
		c1, c2 color.RGBA
	}{
		{"2026-05-02 09:12:00", "DemoCam X1", 39.9042, 116.4074, "BEIJING MORNING", color.RGBA{0x1b, 0x3a, 0x5c, 255}, color.RGBA{0xf2, 0x8e, 0x5b, 255}},
		{"2026-05-02 18:40:00", "DemoCam X1", 39.9163, 116.3972, "FORBIDDEN CITY", color.RGBA{0x5b, 0x1f, 0x2a, 255}, color.RGBA{0xe8, 0xc0, 0x6a, 255}},
		{"2026-04-18 14:05:00", "PhoneCam 15", 31.2304, 121.4737, "SHANGHAI BUND", color.RGBA{0x0f, 0x2b, 0x46, 255}, color.RGBA{0x5f, 0xc6, 0xc9, 255}},
		{"2026-04-18 20:22:00", "PhoneCam 15", 31.2397, 121.4998, "NIGHT LIGHTS", color.RGBA{0x14, 0x14, 0x2b, 255}, color.RGBA{0xe0, 0x6c, 0x9f, 255}},
		{"2026-03-07 11:30:00", "DemoCam X1", 30.2741, 120.1551, "WEST LAKE", color.RGBA{0x1f, 0x4d, 0x3a, 255}, color.RGBA{0xbf, 0xd8, 0x9a, 255}},
		{"2026-03-07 16:48:00", "DemoCam X1", 30.2489, 120.1400, "TEA GARDEN", color.RGBA{0x3a, 0x4b, 0x1f, 255}, color.RGBA{0xd9, 0xc1, 0x7a, 255}},
		{"2026-01-21 08:15:00", "PhoneCam 15", 0, 0, "WINTER MORNING", color.RGBA{0x2b, 0x35, 0x45, 255}, color.RGBA{0xb8, 0xc6, 0xd9, 255}},
		{"2025-12-25 19:05:00", "PhoneCam 15", 0, 0, "HOLIDAY EVENING", color.RGBA{0x3b, 0x1b, 0x2b, 255}, color.RGBA{0xf0, 0x9a, 0x6b, 255}},
		{"2025-10-03 10:40:00", "DemoCam X1", 34.3416, 108.9398, "ANCIENT CITY", color.RGBA{0x4a, 0x2c, 0x1a, 255}, color.RGBA{0xd8, 0xb4, 0x7a, 255}},
		{"2025-08-16 17:25:00", "DemoCam X1", 22.5431, 114.0579, "COASTLINE", color.RGBA{0x08, 0x3a, 0x52, 255}, color.RGBA{0x7a, 0xd8, 0xe0, 255}},
		{"2025-08-16 06:55:00", "DemoCam X1", 22.6000, 114.2000, "SUNRISE", color.RGBA{0x2a, 0x1e, 0x4a, 255}, color.RGBA{0xe6, 0x9a, 0x8a, 255}},
		{"2025-06-01 13:10:00", "PhoneCam 15", 0, 0, "SUMMER GARDEN", color.RGBA{0x1c, 0x40, 0x2b, 255}, color.RGBA{0xa8, 0xd1, 0x8f, 255}},
	}
	for i, ph := range photos {
		t, _ := time.ParseInLocation("2006-01-02 15:04:05", ph.date, time.Local)
		dir := filepath.Join(root, t.Format("2006"), t.Format("01"), t.Format("02"))
		_ = os.MkdirAll(dir, 0o700)
		name := fmt.Sprintf("IMG_%04d.jpg", 1001+i)
		abs := filepath.Join(dir, name)
		if err := renderCard(abs, 1200, 900, ph.title, t.Format("2006-01-02 15:04"), ph.c1, ph.c2); err != nil {
			return err
		}
		st, _ := os.Stat(abs)
		var size int64
		if st != nil {
			size = st.Size()
		}
		lat, lng := ph.lat, ph.lng
		p := &Photo{
			OwnerID: u.ID, Path: abs, FileName: name, SizeBytes: size,
			Width: 1200, Height: 900, TakenAt: t.UTC().Format(time.RFC3339),
			Camera: ph.camera, Lens: "24-70mm f/2.8", GPSLat: lat, GPSLng: lng,
			Orientation: 1, CreatedAt: Now(),
		}
		if err := s.UpsertPhoto(p); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) seedDemoMovies(cfg *config.Config, u *User) error {
	posterDir := filepath.Join(cfg.CacheDir, "demo-posters")
	_ = os.MkdirAll(posterDir, 0o700)
	movies := []struct {
		title, original, overview, genres string
		year                              int
		kind                              string
		rating                            float64
		runtime                           int
		c1, c2                            color.RGBA
	}{
		{"星际回响", "Echoes of the Stars", "一支深空探测队在木星轨道发现异常信号，随之而来的是一段关于人类记忆的旅程。", "科幻,冒险", 2025, "movie", 8.6, 7280, color.RGBA{0x10, 0x1c, 0x3a, 255}, color.RGBA{0x5b, 0x8f, 0xd8, 255}},
		{"雾中灯塔", "Lighthouse in the Mist", "海边小镇的灯塔看守人，在浓雾中等待一艘三十年未归的船。", "剧情,悬疑", 2024, "movie", 8.1, 6420, color.RGBA{0x1c, 0x2a, 0x2e, 255}, color.RGBA{0xc9, 0xb4, 0x76, 255}},
		{"边界之外", "Beyond the Border", "纪录片团队穿越无人区，记录冰川消融与牧民迁徙的真实影像。", "纪录片", 2026, "documentary", 9.0, 5100, color.RGBA{0x1b, 0x33, 0x2a, 255}, color.RGBA{0x9f, 0xc9, 0xa8, 255}},
		{"代码黎明", "Code Dawn", "一群工程师试图让旧机器重新运行，却唤醒了被遗忘的协议。", "科幻,剧集", 2025, "series", 8.9, 2880, color.RGBA{0x24, 0x12, 0x2e, 255}, color.RGBA{0xb0, 0x6c, 0xd8, 255}},
		{"夏日列车", "Summer Train", "一列穿过稻田的慢车，载着少年们最后一次暑假旅行。", "动画,治愈", 2023, "anime", 8.4, 5760, color.RGBA{0x1d, 0x3a, 0x4a, 255}, color.RGBA{0xf4, 0xd0, 0x8a, 255}},
		{"静默档案", "The Silent Archive", "档案管理员在销毁的磁带里发现一段不该存在的录音。", "悬疑,惊悚", 2024, "movie", 7.8, 6600, color.RGBA{0x28, 0x28, 0x28, 255}, color.RGBA{0xa8, 0x3a, 0x3a, 255}},
	}
	for i, m := range movies {
		poster := filepath.Join(posterDir, fmt.Sprintf("demo-%02d.jpg", i+1))
		if err := renderCard(poster, 400, 600, m.original, fmt.Sprintf("%d | %.1f", m.year, m.rating), m.c1, m.c2); err != nil {
			return err
		}
		fileName := fmt.Sprintf("demo-%02d.mp4", i+1)
		mv := &Movie{
			OwnerID: u.ID, Path: filepath.Join(cfg.DataDir, "movies", u.Username, fileName),
			FileName: fileName, Title: m.title, OriginalTitle: m.original,
			Year: m.year, Kind: m.kind, Overview: m.overview, Poster: poster, Rating: m.rating,
			RuntimeSeconds: m.runtime, Genres: m.genres, Cast: "演示数据 · 未关联媒体文件",
			Scraped: true, Resolution: "1920x1080", VideoCodec: "h264", AudioCodec: "aac",
			CreatedAt: Now(),
		}
		if _, err := s.db.Exec(`INSERT INTO movies (owner_id, path, file_name, size_bytes, title, original_title,
			year, kind, season, episode, overview, poster, backdrop, rating, runtime_seconds, genres,
			cast_list, tmdb_id, resolution, video_codec, audio_codec, scraped, created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			mv.OwnerID, mv.Path, mv.FileName, mv.SizeBytes, mv.Title, mv.OriginalTitle, mv.Year, mv.Kind,
			0, 0, mv.Overview, mv.Poster, "", mv.Rating, mv.RuntimeSeconds, mv.Genres, mv.Cast,
			0, mv.Resolution, mv.VideoCodec, mv.AudioCodec, mv.Scraped, mv.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) seedDemoActivity(u *User) {
	entries := []struct {
		offset  time.Duration
		cat     string
		action  string
		target  string
		detail  string
	}{
		{2 * time.Minute, "auth", "login", "-", "登录成功"},
		{38 * time.Minute, "file", "upload", "Documents/隐私清单.txt", "上传文件"},
		{2 * time.Hour, "photo", "upload", "IMG_1001.jpg", "新增照片"},
		{5 * time.Hour, "system", "smart_test", "/dev/sda", "SMART 短测试通过"},
		{9 * time.Hour, "docker", "container_start", "privacyos-jellyfin", "启动容器"},
		{26 * time.Hour, "auth", "login_failed", "-", "密码错误"},
		{30 * time.Hour, "file", "mkdir", "Projects", "新建目录"},
		{49 * time.Hour, "user", "quota_update", "demo", "配额调整为 200 GB"},
	}
	for _, e := range entries {
		_ = s.Audit(&AuditLog{
			CreatedAt: time.Now().Add(-e.offset).UTC().Format(time.RFC3339),
			UserID:    u.ID, Username: u.Username, Category: e.cat, Action: e.action,
			Target: e.target, Detail: e.detail, IP: "192.168.1.20",
			Result: map[bool]string{true: "ok", false: "failed"}[e.action != "login_failed"],
		})
	}
	_ = s.RecordAttempt(u.Username, "192.168.1.20", true, "password")
	_ = s.RecordAttempt(u.Username, "192.168.1.20", true, "password")
	_ = s.RecordAttempt(u.Username, "192.168.1.99", false, "bad_password")
	_ = s.Notify("info", "欢迎使用 PrivacyOS", "这是开发演示环境，数据位于 dev-data/ 目录。")
	_ = s.Notify("warning", "磁盘 SMART 提醒", "磁盘 /dev/sdb 的重新分配扇区计数有所增加，建议尽快备份。")
	_ = s.Notify("info", "系统更新可用", "检测到 3 个软件包可更新。")
	_ = s.RaiseAlert("smart-sdb", "warning", "磁盘健康告警", "磁盘 /dev/sdb 的 SMART 指标异常，请检查。")
}

// renderCard draws a gradient placeholder card with a label and writes it as JPEG.
func renderCard(path string, w, h int, title, subtitle string, c1, c2 color.RGBA) error {
	smallW, smallH := 200, h*200/w
	img := image.NewRGBA(image.Rect(0, 0, smallW, smallH))
	for y := 0; y < smallH; y++ {
		for x := 0; x < smallW; x++ {
			f := float64(x+y) / float64(smallW+smallH)
			col := color.RGBA{
				R: uint8(float64(c1.R)*(1-f) + float64(c2.R)*f),
				G: uint8(float64(c1.G)*(1-f) + float64(c2.G)*f),
				B: uint8(float64(c1.B)*(1-f) + float64(c2.B)*f),
				A: 255,
			}
			img.Set(x, y, col)
		}
	}
	// Soft overlay band for legibility.
	band := image.Rect(0, smallH*3/4, smallW, smallH)
	draw.Draw(img, band, &image.Uniform{color.RGBA{0, 0, 0, 120}}, image.Point{}, draw.Over)

	d := &font.Drawer{Dst: img, Src: image.NewUniform(color.RGBA{255, 255, 255, 255})}
	face := basicfont.Face7x13
	d.Face = face
	lines := wrapLabel(title, 24)
	subtitleY := smallH - 10
	ty := subtitleY - 18 - (len(lines)-1)*16
	for _, line := range lines {
		d.Dot = fixed.P(8, ty)
		d.DrawString(line)
		ty += 16
	}
	d.Src = image.NewUniform(color.RGBA{235, 235, 235, 255})
	d.Dot = fixed.P(8, subtitleY)
	d.DrawString(subtitle)

	final := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.NearestNeighbor.Scale(final, final.Bounds(), img, img.Bounds(), draw.Over, nil)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, final, &jpeg.Options{Quality: 86})
}

func wrapLabel(s string, width int) []string {
	var out []string
	for len(s) > width {
		cut := strings.LastIndex(s[:width], " ")
		if cut <= 0 {
			cut = width
		}
		out = append(out, strings.TrimSpace(s[:cut]))
		s = strings.TrimSpace(s[cut:])
	}
	if s != "" {
		out = append(out, s)
	}
	return out
}

