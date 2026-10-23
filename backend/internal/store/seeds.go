package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/bcrypt"

	"privacyos/backend/internal/config"
)

// seed creates the initial administrator, application catalog and default
// network shares on a fresh installation.
func (s *Store) seed(cfg *config.Config) error {
	if err := s.seedAdmin(cfg); err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}
	if err := s.seedApps(); err != nil {
		return fmt.Errorf("seed apps: %w", err)
	}
	if err := s.seedNetworkShares(cfg); err != nil {
		return fmt.Errorf("seed shares: %w", err)
	}
	if cfg.Dev && os.Getenv("PRIVACYOS_DEMO") != "0" {
		if err := s.seedDemo(cfg); err != nil {
			return fmt.Errorf("seed demo: %w", err)
		}
	}
	return nil
}

func (s *Store) seedAdmin(cfg *config.Config) error {
	n, err := s.CountUsers("")
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	admin := &User{
		Username:     cfg.AdminUser,
		DisplayName:  "PrivacyOS Admin",
		PasswordHash: string(hash),
		Role:         "admin",
		Status:       "active",
		Permissions:  DefaultPermissions("admin"),
		Preferences:  Preferences{Lang: "zh-CN", Theme: "dark"},
	}
	if err := s.CreateUser(admin); err != nil {
		return err
	}
	for _, dir := range []string{
		filepath.Join(cfg.DataDir, "users", admin.Username),
		filepath.Join(cfg.DataDir, "photos", admin.Username),
		filepath.Join(cfg.DataDir, "movies", admin.Username),
	} {
		_ = os.MkdirAll(dir, 0o700)
	}
	return nil
}

func (s *Store) seedApps() error {
	for _, app := range catalogApps {
		ports, _ := json.Marshal(app.Ports)
		_, err := s.db.Exec(`INSERT INTO apps (slug, name, category, description, icon, image, ports, compose, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?)
			ON CONFLICT(slug) DO UPDATE SET
				name=excluded.name, category=excluded.category, description=excluded.description,
				icon=excluded.icon, image=excluded.image, ports=excluded.ports, compose=excluded.compose`,
			app.Slug, app.Name, app.Category, app.Description, app.Icon, app.Image, string(ports), app.Compose, Now())
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) seedNetworkShares(cfg *config.Config) error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM network_shares`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	defaults := []NetworkShare{
		{Name: "users", Path: filepath.Join(cfg.DataDir, "users"), Comment: "用户文件", Protocols: []string{"smb", "nfs"}, ValidUsers: "@users"},
		{Name: "photos", Path: filepath.Join(cfg.DataDir, "photos"), Comment: "照片", Protocols: []string{"smb"}, ValidUsers: "@users"},
		{Name: "movies", Path: filepath.Join(cfg.DataDir, "movies"), Comment: "影视", Protocols: []string{"smb"}, ValidUsers: "@users"},
	}
	for _, d := range defaults {
		if err := s.CreateNetworkShare(&d); err != nil {
			return err
		}
	}
	return nil
}
