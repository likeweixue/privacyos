package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"privacyos/backend/internal/store"
)

// VaultInfo describes a per-user encrypted home directory.
type VaultInfo struct {
	Username  string `json:"username"`
	Exists    bool   `json:"exists"`
	Unlocked  bool   `json:"unlocked"`
	Available bool   `json:"available"`
	Device    string `json:"device,omitempty"`
	SizeBytes int64  `json:"size_bytes"`
	Method    string `json:"method"`
}

var vaultMu sync.Mutex

// VaultFile returns the LUKS container path for a user.
func (a *App) VaultFile(username string) string {
	return filepath.Join(a.Cfg.DataDir, "vaults", username+".luks")
}

// vaultKeyFile returns the recovery key path for a user vault.
func (a *App) vaultKeyFile(username string) string {
	return filepath.Join(a.Cfg.ConfigDir, "vault-keys", username+".key")
}

// vaultMapper returns the device-mapper name.
func (a *App) vaultMapper(username string) string {
	clean := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, username)
	return "privacyos-" + clean
}

// VaultExists reports whether a user has an encrypted home.
func (a *App) VaultExists(username string) bool {
	st, err := os.Stat(a.VaultFile(username))
	return err == nil && st.Mode().IsRegular()
}

// VaultAvailable reports whether LUKS tooling can be used on this host.
func (a *App) VaultAvailable() bool {
	if a.Cfg.Dev || runtime.GOOS != "linux" {
		return false
	}
	_, err := exec.LookPath("cryptsetup")
	return err == nil
}

// VaultInfo returns the state of a user's vault.
func (a *App) VaultInfo(username string) VaultInfo {
	info := VaultInfo{Username: username, Method: "luks2", Available: a.VaultAvailable()}
	if st, err := os.Stat(a.VaultFile(username)); err == nil {
		info.Exists = true
		info.SizeBytes = st.Size()
		info.Device = a.VaultFile(username)
	}
	if _, err := os.Stat(filepath.Join(a.Cfg.DataDir, "vaults", username+".unlocked")); err == nil {
		info.Unlocked = true
	}
	return info
}

// CreateVault formats a sparse LUKS2 container and mounts it on the user's home.
func (a *App) CreateVault(ctx context.Context, u *store.User, sizeBytes int64) error {
	vaultMu.Lock()
	defer vaultMu.Unlock()

	if a.VaultExists(u.Username) {
		return fmt.Errorf("加密卷已存在")
	}
	if sizeBytes < 1024*1024*1024 {
		sizeBytes = 1024 * 1024 * 1024
	}
	if err := os.MkdirAll(filepath.Join(a.Cfg.DataDir, "vaults"), 0o700); err != nil {
		return err
	}
	keyDir := filepath.Join(a.Cfg.ConfigDir, "vault-keys")
	if err := os.MkdirAll(keyDir, 0o700); err != nil {
		return err
	}
	key, err := RandomKey(32)
	if err != nil {
		return err
	}
	keyPath := a.vaultKeyFile(u.Username)
	if err := os.WriteFile(keyPath, []byte(key+"\n"), 0o600); err != nil {
		return err
	}

	if a.Cfg.Dev || !a.VaultAvailable() {
		if err := os.WriteFile(a.VaultFile(u.Username), []byte("privacyos-dev-vault\n"), 0o600); err != nil {
			return err
		}
		_ = os.WriteFile(filepath.Join(a.Cfg.DataDir, "vaults", u.Username+".unlocked"), []byte("1\n"), 0o600)
		return nil
	}

	file := a.VaultFile(u.Username)
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	if err := f.Truncate(sizeBytes); err != nil {
		f.Close()
		return err
	}
	f.Close()
	if err := os.Chmod(file, 0o600); err != nil {
		return err
	}
	if out, err := a.runCmd(ctx, "cryptsetup", "luksFormat", "--batch-mode", "--type", "luks2",
		"--key-file", keyPath, "--cipher", "aes-xts-plain64", "--key-size", "512",
		"--hash", "sha256", file); err != nil {
		_ = os.Remove(file)
		return fmt.Errorf("luksFormat 失败: %v %s", err, out)
	}
	if err := a.openVault(ctx, u.Username, keyPath); err != nil {
		return err
	}
	if out, err := a.runCmd(ctx, "mkfs.ext4", "-q", "-F", "/dev/mapper/"+a.vaultMapper(u.Username)); err != nil {
		return fmt.Errorf("mkfs 失败: %v %s", err, out)
	}
	mountpoint := filepath.Join(a.Cfg.DataDir, "users", u.Username)
	if err := os.MkdirAll(mountpoint, 0o700); err != nil {
		return err
	}
	if out, err := a.runCmd(ctx, "mount", "/dev/mapper/"+a.vaultMapper(u.Username), mountpoint); err != nil {
		return fmt.Errorf("挂载失败: %v %s", err, out)
	}
	_ = os.Chmod(mountpoint, 0o700)
	return nil
}

// UnlockVault opens and mounts an existing user vault.
func (a *App) UnlockVault(ctx context.Context, username string) error {
	vaultMu.Lock()
	defer vaultMu.Unlock()
	if !a.VaultExists(username) {
		return fmt.Errorf("加密卷不存在")
	}
	if a.Cfg.Dev || !a.VaultAvailable() {
		_ = os.WriteFile(filepath.Join(a.Cfg.DataDir, "vaults", username+".unlocked"), []byte("1\n"), 0o600)
		return nil
	}
	return a.openVault(ctx, username, a.vaultKeyFile(username))
}

func (a *App) openVault(ctx context.Context, username, keyPath string) error {
	mapper := a.vaultMapper(username)
	if _, err := a.runCmd(ctx, "cryptsetup", "status", mapper); err != nil {
		if out, err := a.runCmd(ctx, "cryptsetup", "open", "--key-file", keyPath,
			a.VaultFile(username), mapper); err != nil {
			return fmt.Errorf("打开加密卷失败: %v %s", err, out)
		}
	}
	mountpoint := filepath.Join(a.Cfg.DataDir, "users", username)
	if err := os.MkdirAll(mountpoint, 0o700); err != nil {
		return err
	}
	if _, err := a.runCmd(ctx, "mountpoint", "-q", mountpoint); err != nil {
		if out, err := a.runCmd(ctx, "mount", "/dev/mapper/"+mapper, mountpoint); err != nil {
			return fmt.Errorf("挂载失败: %v %s", err, out)
		}
	}
	return nil
}

// LockVault unmounts and closes a user vault.
func (a *App) LockVault(ctx context.Context, username string) error {
	vaultMu.Lock()
	defer vaultMu.Unlock()
	if a.Cfg.Dev || !a.VaultAvailable() {
		_ = os.Remove(filepath.Join(a.Cfg.DataDir, "vaults", username+".unlocked"))
		return nil
	}
	mountpoint := filepath.Join(a.Cfg.DataDir, "users", username)
	_, _ = a.runCmd(ctx, "umount", mountpoint)
	if _, err := a.runCmd(ctx, "cryptsetup", "close", a.vaultMapper(username)); err != nil {
		return err
	}
	return nil
}

// DestroyVault irreversibly erases all LUKS keyslots and deletes the key file.
// The ciphertext becomes permanently unreadable.
func (a *App) DestroyVault(ctx context.Context, u *store.User) (string, error) {
	vaultMu.Lock()
	defer vaultMu.Unlock()

	file := a.VaultFile(u.Username)
	keyPath := a.vaultKeyFile(u.Username)
	if a.Cfg.Dev || !a.VaultAvailable() {
		_ = os.Remove(filepath.Join(a.Cfg.DataDir, "vaults", u.Username+".unlocked"))
		_ = os.Remove(file)
		_ = os.Remove(keyPath)
		return "已销毁加密卷密钥（开发模拟）", nil
	}

	mountpoint := filepath.Join(a.Cfg.DataDir, "users", u.Username)
	_, _ = a.runCmd(ctx, "umount", "-l", mountpoint)
	_, _ = a.runCmd(ctx, "cryptsetup", "close", a.vaultMapper(u.Username))

	if keyPath != "" {
		if info, err := os.Stat(keyPath); err == nil {
			_ = overwriteFile(keyPath, info.Size())
			_ = os.Remove(keyPath)
		}
	}
	out, err := a.runCmd(ctx, "cryptsetup", "luksErase", "--batch-mode", file)
	if err != nil {
		return "", fmt.Errorf("luksErase 失败: %v %s", err, out)
	}
	if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return "LUKS 密钥槽已全部擦除，密文不可恢复", nil
}

func (a *App) runCmd(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
