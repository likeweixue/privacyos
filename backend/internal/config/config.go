// Package config loads the PrivacyOS runtime configuration.
//
// Configuration is resolved in this order: built-in defaults, then the edition
// marker file (/opt/privacyos/edition or $PRIVACYOS_CONFIG_DIR/edition), then
// environment variables. Development mode (PRIVACYOS_DEV=1, or any non-Linux
// host) keeps every path inside ./dev-data so the panel can be exercised on a
// workstation without touching the real system.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"privacyos/backend/internal/version"
)

// Config is the fully resolved runtime configuration.
type Config struct {
	Version   string
	Listen    string
	Edition   string // "normal" or "pro"
	Dev       bool
	DataDir   string // /data
	ConfigDir string // /etc/privacyos
	LogDir    string // /var/log/privacyos
	CacheDir  string // thumbnails, posters, temp archives
	DBPath    string
	WebDir    string
	Secret    []byte
	TLSCert   string
	TLSKey    string

	// AdminPassword is only used when the database is seeded for the first time.
	AdminPassword string
	// AdminUser is the first account created on a fresh installation.
	AdminUser string
}

// Pro reports whether this installation is the Professional edition.
func (c *Config) Pro() bool { return c.Edition == "pro" }

// EditionName returns the localized edition label.
func (c *Config) EditionName() string {
	if c.Pro() {
		return "专业版"
	}
	return "普通版"
}

// Load resolves the configuration and creates the runtime directories.
func Load() (*Config, error) {
	dev := os.Getenv("PRIVACYOS_DEV") == "1" || runtime.GOOS != "linux"
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	c := &Config{
		Version:       version.Version,
		Listen:        ":80",
		Edition:       "normal",
		Dev:           dev,
		DataDir:       "/data",
		ConfigDir:     "/etc/privacyos",
		LogDir:        "/var/log/privacyos",
		WebDir:        "/opt/privacyos/web",
		AdminUser:     "padmin",
		AdminPassword: "privacyos",
	}

	if dev {
		base := filepath.Join(cwd, "dev-data")
		c.Listen = "127.0.0.1:8080"
		c.DataDir = filepath.Join(base, "data")
		c.ConfigDir = filepath.Join(base, "etc")
		c.LogDir = filepath.Join(base, "log")
		c.WebDir = filepath.Join(cwd, "..", "frontend", "dist")
	}
	c.CacheDir = filepath.Join(c.DataDir, ".cache")

	// Edition marker written by the installer.
	if edition := firstNonEmpty(
		os.Getenv("PRIVACYOS_EDITION"),
		readTrimmed(filepath.Join(c.ConfigDir, "edition")),
		readTrimmed("/opt/privacyos/edition"),
	); edition != "" {
		c.Edition = strings.ToLower(edition)
	}
	if c.Edition != "pro" {
		c.Edition = "normal"
	}

	// Environment overrides.
	applyEnv := func(key string, dst *string) {
		if v := os.Getenv(key); v != "" {
			*dst = v
		}
	}
	applyEnv("PRIVACYOS_LISTEN", &c.Listen)
	applyEnv("PRIVACYOS_DATA_DIR", &c.DataDir)
	applyEnv("PRIVACYOS_CONFIG_DIR", &c.ConfigDir)
	applyEnv("PRIVACYOS_LOG_DIR", &c.LogDir)
	applyEnv("PRIVACYOS_WEB_DIR", &c.WebDir)
	applyEnv("PRIVACYOS_ADMIN_USER", &c.AdminUser)
	applyEnv("PRIVACYOS_ADMIN_PASSWORD", &c.AdminPassword)
	applyEnv("PRIVACYOS_EDITION", &c.Edition)
	c.CacheDir = filepath.Join(c.DataDir, ".cache")

	if cert := os.Getenv("PRIVACYOS_TLS_CERT"); cert != "" {
		c.TLSCert = cert
		c.TLSKey = os.Getenv("PRIVACYOS_TLS_KEY")
	} else {
		defCert := filepath.Join(c.ConfigDir, "tls", "server.crt")
		defKey := filepath.Join(c.ConfigDir, "tls", "server.key")
		if fileExists(defCert) && fileExists(defKey) {
			c.TLSCert, c.TLSKey = defCert, defKey
		}
	}

	for _, dir := range []string{
		c.DataDir, c.ConfigDir, c.LogDir, c.CacheDir,
		filepath.Join(c.DataDir, "users"),
		filepath.Join(c.DataDir, "photos"),
		filepath.Join(c.DataDir, "movies"),
		filepath.Join(c.DataDir, "backups"),
		filepath.Join(c.DataDir, "docker"),
		filepath.Join(c.DataDir, "trash"),
		filepath.Join(c.CacheDir, "thumbs"),
		filepath.Join(c.CacheDir, "posters"),
		filepath.Join(c.CacheDir, "tmp"),
	} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("create %s: %w", dir, err)
		}
	}

	c.DBPath = filepath.Join(c.ConfigDir, "privacyos.db")
	if v := os.Getenv("PRIVACYOS_DB"); v != "" {
		c.DBPath = v
	}

	secret, err := loadOrCreateSecret(filepath.Join(c.ConfigDir, "secret.key"))
	if err != nil {
		return nil, err
	}
	c.Secret = secret
	return c, nil
}

// loadOrCreateSecret returns a persistent 32-byte signing key.
func loadOrCreateSecret(path string) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil {
		if b, derr := hex.DecodeString(strings.TrimSpace(string(data))); derr == nil && len(b) >= 32 {
			return b, nil
		}
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}
	if err := os.WriteFile(path, []byte(hex.EncodeToString(buf)+"\n"), 0o600); err != nil {
		return nil, fmt.Errorf("write secret: %w", err)
	}
	return buf, nil
}

func readTrimmed(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
