// Package core wires the domain configuration, edition rules, background jobs,
// notifications and the security policy engine (lockout / destruction).
package core

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"privacyos/backend/internal/config"
	"privacyos/backend/internal/store"
	"privacyos/backend/internal/version"
)

// App is the shared application context.
type App struct {
	Cfg       *config.Config
	Store     *store.Store
	Jobs      *Jobs
	Notify    *Notifier
	StartedAt time.Time
	HTTP      *http.Client
}

// New creates the application context and seeds default settings.
func New(cfg *config.Config, st *store.Store) (*App, error) {
	a := &App{
		Cfg:       cfg,
		Store:     st,
		Jobs:      NewJobs(),
		Notify:    &Notifier{cfg: cfg, st: st},
		StartedAt: time.Now(),
		HTTP:      &http.Client{Timeout: 20 * time.Second},
	}
	if err := a.EnsureDefaults(); err != nil {
		return nil, fmt.Errorf("seed settings: %w", err)
	}
	return a, nil
}

// Pro reports the Professional edition.
func (a *App) Pro() bool { return a.Cfg.Pro() }

// Version returns the running version.
func (a *App) Version() string { return version.Version }

// ---------- settings ----------

// SecuritySettings configures login failure handling.
type SecuritySettings struct {
	LockoutEnabled   bool `json:"lockout_enabled"`
	MaxAttempts      int  `json:"max_attempts"`
	WindowSeconds    int  `json:"window_seconds"`
	LockSeconds      int  `json:"lock_seconds"`
	DelayEnabled     bool `json:"delay_enabled"`
	DelayFrom        int  `json:"delay_from"`
	BaseDelaySeconds int  `json:"base_delay_seconds"`
	MaxDelaySeconds  int  `json:"max_delay_seconds"`
}

// DestroySettings configures the Professional-edition destruction policy.
type DestroySettings struct {
	Enabled       bool   `json:"enabled"`
	Threshold     int    `json:"threshold"`
	WindowSeconds int    `json:"window_seconds"`
	Scope         string `json:"scope"`
	Method        string `json:"method"`
	Silent        bool   `json:"silent"`
	NotifyAdmin   bool   `json:"notify_admin"`
	VaultDevice   string `json:"vault_device"`
	Fallback      string `json:"fallback"`
}

// DeletionSettings configures cooling periods and trash retention.
type DeletionSettings struct {
	CoolingDays       int `json:"cooling_days"`
	TrashRetentionDays int `json:"trash_retention_days"`
}

// TMDBSettings holds the metadata provider configuration.
type TMDBSettings struct {
	APIKey   string `json:"api_key"`
	Language string `json:"language"`
}

// NotificationSettings holds outbound notification channels.
type NotificationSettings struct {
	EmailEnabled   bool   `json:"email_enabled"`
	SMTPHost       string `json:"smtp_host"`
	SMTPPort       int    `json:"smtp_port"`
	SMTPUser       string `json:"smtp_user"`
	SMTPPassword   string `json:"smtp_password"`
	SMTPFrom       string `json:"smtp_from"`
	SMTPTo         string `json:"smtp_to"`
	SMTPTLS        bool   `json:"smtp_tls"`
	TelegramEnable bool   `json:"telegram_enabled"`
	TelegramToken  string `json:"telegram_token"`
	TelegramChatID string `json:"telegram_chat_id"`
	BarkEnabled    bool   `json:"bark_enabled"`
	BarkURL        string `json:"bark_url"`
	ServerChanKey  string `json:"serverchan_key"`
	Events         []string `json:"events"`
}

// InterfaceConfig is a static/DHCP network interface configuration.
type InterfaceConfig struct {
	Mode    string `json:"mode"`
	Address string `json:"address,omitempty"`
	Gateway string `json:"gateway,omitempty"`
	DNS     string `json:"dns,omitempty"`
}

// NetworkSettings holds host networking preferences.
type NetworkSettings struct {
	Hostname   string                     `json:"hostname"`
	Domain     string                     `json:"domain"`
	DNS        []string                   `json:"dns"`
	Interfaces map[string]InterfaceConfig `json:"interfaces"`
}

// DDNSSettings holds dynamic DNS configuration.
type DDNSSettings struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
	Domain   string `json:"domain"`
	Token    string `json:"token"`
	Username string `json:"username"`
	Password string `json:"password"`
	URL      string `json:"url"`
	Interval int    `json:"interval"`
	LastIP   string `json:"last_ip"`
	LastRun  string `json:"last_run"`
	LastErr  string `json:"last_err"`
}

// SystemSettings holds branding shown on the login page and boot banner.
type SystemSettings struct {
	Name    string `json:"name"`
	Website string `json:"website"`
}

// NormalSettings holds Normal-edition specific switches.
type NormalSettings struct {
	AdminSeeAll bool `json:"admin_see_all"`
}

// HTTPSSettings holds TLS configuration.
type HTTPSSettings struct {
	Enabled bool   `json:"enabled"`
	Mode    string `json:"mode"`
	Domain  string `json:"domain"`
	Cert    string `json:"cert"`
	Key     string `json:"key"`
}

// BackupSettings holds scheduled configuration backup preferences.
type BackupSettings struct {
	Auto     bool `json:"auto"`
	Keep     int  `json:"keep"`
	Schedule string `json:"schedule"`
}

// Settings is the aggregate configuration view.
type Settings struct {
	Security      SecuritySettings      `json:"security"`
	Destroy       DestroySettings       `json:"destroy"`
	Deletion      DeletionSettings      `json:"deletion"`
	TMDB          TMDBSettings          `json:"tmdb"`
	Notifications NotificationSettings  `json:"notifications"`
	Network       NetworkSettings       `json:"network"`
	DDNS          DDNSSettings          `json:"ddns"`
	System        SystemSettings        `json:"system"`
	Normal        NormalSettings        `json:"normal"`
	HTTPS         HTTPSSettings         `json:"https"`
	Backup        BackupSettings        `json:"backup"`
}

// DefaultSettings returns the factory defaults.
func DefaultSettings() Settings {
	return Settings{
		Security: SecuritySettings{
			LockoutEnabled: true, MaxAttempts: 10, WindowSeconds: 3600,
			LockSeconds: 900, DelayEnabled: true, DelayFrom: 3,
			BaseDelaySeconds: 3, MaxDelaySeconds: 300,
		},
		Destroy: DestroySettings{
			Enabled: false, Threshold: 10, WindowSeconds: 3600, Scope: "user",
			Method: "key_destroy", Silent: true, NotifyAdmin: false,
			Fallback: "overwrite",
		},
		Deletion:      DeletionSettings{CoolingDays: 30, TrashRetentionDays: 30},
		TMDB:          TMDBSettings{Language: "zh-CN"},
		Notifications: NotificationSettings{SMTPPort: 587, SMTPTLS: true, Events: []string{"login", "destroy", "smart"}},
		Network:       NetworkSettings{Hostname: "privacyos", Domain: "local", DNS: []string{"223.5.5.5", "1.1.1.1"}, Interfaces: map[string]InterfaceConfig{}},
		DDNS:          DDNSSettings{Provider: "duckdns", Interval: 300},
		System:        SystemSettings{Name: "PrivacyOS", Website: "privacyos.local"},
		Normal:        NormalSettings{AdminSeeAll: true},
		HTTPS:         HTTPSSettings{Enabled: false, Mode: "selfsigned"},
		Backup:        BackupSettings{Keep: 7, Schedule: "daily"},
	}
}

// EnsureDefaults writes any missing settings keys.
func (a *App) EnsureDefaults() error {
	d := DefaultSettings()
	fields := map[string]any{
		"security": d.Security, "destroy": d.Destroy, "deletion": d.Deletion,
		"tmdb": d.TMDB, "notifications": d.Notifications, "network": d.Network,
		"ddns": d.DDNS, "system": d.System, "normal": d.Normal,
		"https": d.HTTPS, "backup": d.Backup,
	}
	for k, v := range fields {
		if err := a.Store.DefaultSetting(k, v); err != nil {
			return err
		}
	}
	return nil
}

// LoadSettings returns the aggregate settings with defaults applied.
func (a *App) LoadSettings() Settings {
	s := DefaultSettings()
	_, _ = a.Store.Setting("security", &s.Security)
	_, _ = a.Store.Setting("destroy", &s.Destroy)
	_, _ = a.Store.Setting("deletion", &s.Deletion)
	_, _ = a.Store.Setting("tmdb", &s.TMDB)
	_, _ = a.Store.Setting("notifications", &s.Notifications)
	_, _ = a.Store.Setting("network", &s.Network)
	_, _ = a.Store.Setting("ddns", &s.DDNS)
	_, _ = a.Store.Setting("system", &s.System)
	_, _ = a.Store.Setting("normal", &s.Normal)
	_, _ = a.Store.Setting("https", &s.HTTPS)
	_, _ = a.Store.Setting("backup", &s.Backup)
	if s.Network.Interfaces == nil {
		s.Network.Interfaces = map[string]InterfaceConfig{}
	}
	if s.Destroy.Threshold < 3 {
		s.Destroy.Threshold = 3
	}
	if s.Security.MaxAttempts < 3 {
		s.Security.MaxAttempts = 3
	}
	return s
}

// AdminSeeAll reports whether administrators may browse other users' files.
// This is only possible in the Normal edition with the switch enabled.
func (a *App) AdminSeeAll() bool {
	if a.Pro() {
		return false
	}
	s := a.LoadSettings()
	return s.Normal.AdminSeeAll
}

// CanSeeOtherFiles reports whether u may browse another user's home.
func (a *App) CanSeeOtherFiles(u *store.User) bool {
	return u != nil && u.Role == "admin" && a.AdminSeeAll()
}

// FeatureAllowed checks a per-user feature permission.
func (a *App) FeatureAllowed(u *store.User, feature string) bool {
	if u == nil {
		return false
	}
	switch feature {
	case "files":
		return u.Permissions.Files
	case "photos":
		return u.Permissions.Photos
	case "movies":
		return u.Permissions.Movies
	case "share":
		return u.Permissions.Share
	case "docker":
		return u.Permissions.Docker
	case "apps":
		return u.Permissions.Apps
	}
	return false
}

// StartupChecks raises alerts for insecure defaults.
func (a *App) StartupChecks() {
	if a.Cfg.AdminPassword == "privacyos" {
		_ = a.Store.Notify("warning", "安全提醒：默认密码未修改",
			"管理员账号仍在使用安装默认密码，请立即在“系统设置 → 账户安全”中修改。")
		_ = a.Store.RaiseAlert("default-password", "warning", "默认密码风险", "管理员账号使用默认密码 privacyos。")
	}
	if !a.Cfg.Pro() {
		_ = a.Store.ResolveAlert("luks-disabled")
	}
}

// CleanupLoop periodically purges expired sessions, trash and accounts.
func (a *App) CleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.RunJanitor()
		}
	}
}

// ---------- helpers ----------

// ContainsString reports whether list contains s.
func ContainsString(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}
