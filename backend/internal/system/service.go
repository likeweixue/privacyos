// Package system provides host information, updates, backups, TLS, firewall,
// encryption status and Linux account provisioning.
package system

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"privacyos/backend/internal/config"
	"privacyos/backend/internal/store"
	"privacyos/backend/internal/version"
)

// Service exposes system-level operations.
type Service struct {
	Cfg   *config.Config
	Store *store.Store
}

// New creates a system service.
func New(cfg *config.Config, st *store.Store) *Service {
	return &Service{Cfg: cfg, Store: st}
}

// Dev reports whether privileged operations are simulated.
func (s *Service) Dev() bool { return s.Cfg.Dev || runtime.GOOS != "linux" }

// Info is the system information panel payload.
type Info struct {
	Version       string   `json:"version"`
	Edition       string   `json:"edition"`
	EditionName   string   `json:"edition_name"`
	OS            string   `json:"os"`
	Kernel        string   `json:"kernel"`
	Arch          string   `json:"arch"`
	Hostname      string   `json:"hostname"`
	Uptime        int64    `json:"uptime"`
	BootTime      string   `json:"boot_time"`
	CPUModel      string   `json:"cpu_model"`
	Cores         int      `json:"cores"`
	MemTotal      int64    `json:"mem_total"`
	IPs           []string `json:"ips"`
	WebURL        string   `json:"web_url"`
	Encryption    EncryptionInfo `json:"encryption"`
	Capabilities  Capabilities   `json:"capabilities"`
	Dev           bool     `json:"dev"`
	StartedAt     string   `json:"started_at"`
}

// Capabilities reports which optional host components are installed.
type Capabilities struct {
	Samba      bool `json:"samba"`
	NFS        bool `json:"nfs"`
	Docker     bool `json:"docker"`
	FFmpeg     bool `json:"ffmpeg"`
	Smartctl   bool `json:"smartctl"`
	Cryptsetup bool `json:"cryptsetup"`
	Mdadm      bool `json:"mdadm"`
	Nft        bool `json:"nft"`
}

// DetectCapabilities probes optional binaries.
func DetectCapabilities(dev bool) Capabilities {
	if dev {
		return Capabilities{Samba: true, NFS: true, Docker: true, FFmpeg: true,
			Smartctl: true, Cryptsetup: true, Mdadm: true, Nft: true}
	}
	has := func(name string) bool {
		_, err := exec.LookPath(name)
		return err == nil
	}
	return Capabilities{
		Samba: has("smbd"), NFS: has("exportfs"), Docker: has("docker"),
		FFmpeg: has("ffmpeg"), Smartctl: has("smartctl"), Cryptsetup: has("cryptsetup"),
		Mdadm: has("mdadm"), Nft: has("nft"),
	}
}

// Info collects system information.
func (s *Service) Info(ctx context.Context) Info {
	info := Info{
		Version: version.Version, Edition: s.Cfg.Edition, EditionName: s.Cfg.EditionName(),
		Arch: runtime.GOARCH, Dev: s.Dev(),
	}
	info.Hostname, _ = os.Hostname()
	info.Cores = runtime.NumCPU()
	info.OS = osPrettyName()
	info.Kernel = kernelVersion()
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		var up float64
		fmt.Sscanf(string(data), "%f", &up)
		info.Uptime = int64(up)
		info.BootTime = time.Now().Add(-time.Duration(up) * time.Second).UTC().Format(time.RFC3339)
	}
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				f := strings.Fields(line)
				if len(f) > 1 {
					kb, _ := strconv.ParseInt(f[1], 10, 64)
					info.MemTotal = kb * 1024
				}
			}
		}
	}
	if s.Dev() {
		info.OS = "Debian GNU/Linux 13.7 (trixie) · 开发模式"
		info.Kernel = "6.12.30-amd64"
		info.CPUModel = "Apple M2 (模拟 8 核)"
		info.MemTotal = 16 << 30
		info.Uptime = 3*86400 + 4211
		info.BootTime = time.Now().Add(-time.Duration(info.Uptime) * time.Second).UTC().Format(time.RFC3339)
	} else {
		info.CPUModel = cpuModel()
	}
	info.IPs = localIPs()
	info.WebURL = "http://" + firstIP(info.IPs)
	info.Encryption = s.Encryption(ctx)
	info.Capabilities = DetectCapabilities(s.Dev())
	info.StartedAt = time.Now().UTC().Format(time.RFC3339)
	return info
}

// PackageUpdate is one available apt upgrade.
type PackageUpdate struct {
	Name    string `json:"name"`
	Current string `json:"current"`
	New     string `json:"new"`
}

// Updates lists available apt upgrades.
func (s *Service) Updates(ctx context.Context) ([]PackageUpdate, error) {
	if s.Dev() {
		return []PackageUpdate{
			{Name: "openssl", Current: "3.5.1-1", New: "3.5.3-1"},
			{Name: "samba", Current: "4.22.2+dfsg-1", New: "4.22.4+dfsg-1"},
			{Name: "docker.io", Current: "27.3.1+dfsg1-1", New: "27.5.0+dfsg1-1"},
		}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "apt-get", "-s", "-o", "Debug::NoLocking=1", "upgrade")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("apt-get 查询失败: %s", strings.TrimSpace(string(out)))
	}
	var updates []PackageUpdate
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "Inst ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		item := PackageUpdate{Name: fields[1]}
		for i, f := range fields {
			if strings.HasPrefix(f, "[") && i+1 < len(fields) {
				item.Current = strings.Trim(fields[i+1], "]")
			}
			if strings.HasPrefix(f, "(") && i+1 < len(fields) {
				item.New = strings.Trim(fields[i+1], "()")
			}
		}
		updates = append(updates, item)
	}
	return updates, nil
}

// ApplyUpdates runs a full system upgrade.
func (s *Service) ApplyUpdates(ctx context.Context, report func(int, string)) error {
	if s.Dev() {
		for i := 0; i <= 100; i += 20 {
			report(i, fmt.Sprintf("模拟更新中 %d%%", i))
			time.Sleep(300 * time.Millisecond)
		}
		return nil
	}
	report(5, "刷新软件源")
	cmd := exec.CommandContext(ctx, "apt-get", "update")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("apt-get update 失败: %s", strings.TrimSpace(string(out)))
	}
	report(20, "开始升级")
	cmd = exec.CommandContext(ctx, "apt-get", "-y", "-o", "Dpkg::Options::=--force-confdef",
		"-o", "Dpkg::Options::=--force-confold", "upgrade")
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("升级失败: %s", strings.TrimSpace(string(out)))
	}
	report(100, "升级完成")
	return nil
}

// CreateBackup archives the configuration directory with a consistent database
// snapshot (SQLite VACUUM INTO), avoiding WAL inconsistency.
func (s *Service) CreateBackup() (string, error) {
	dir := filepath.Join(s.Cfg.CacheDir, "tmp")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	name := "privacyos-backup-" + time.Now().Format("20060102-150405") + ".tar.gz"
	target := filepath.Join(dir, name)

	snapshot := filepath.Join(dir, "snapshot-"+time.Now().Format("150405.000000")+".db")
	if _, err := s.Store.DB().Exec("VACUUM INTO ?", snapshot); err != nil {
		snapshot = ""
		_, _ = s.Store.DB().Exec("PRAGMA wal_checkpoint(FULL)")
	} else {
		defer os.Remove(snapshot)
	}

	f, err := os.Create(target)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	manifest, _ := json.MarshalIndent(map[string]any{
		"created_at": store.Now(), "version": version.Version, "edition": s.Cfg.Edition,
	}, "", "  ")
	_ = tw.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0o600, Size: int64(len(manifest)), ModTime: time.Now()})
	_, _ = tw.Write(manifest)

	err = filepath.Walk(s.Cfg.ConfigDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(s.Cfg.ConfigDir, path)
		if err != nil || rel == "." {
			return nil
		}
		if strings.HasPrefix(rel, "tls") { // certificates are host-specific
			return nil
		}
		// The live database is replaced by the consistent snapshot below.
		if rel == "privacyos.db" || rel == "privacyos.db-wal" || rel == "privacyos.db-shm" {
			return nil
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return nil
		}
		header.Name = filepath.ToSlash(filepath.Join("privacyos", rel))
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			_, err = tw.Write(data)
			return err
		}
		return nil
	})
	if err != nil {
		tw.Close()
		gz.Close()
		os.Remove(target)
		return "", err
	}

	// Append the database snapshot under its real name.
	if err := appendFileToTar(tw, snapshot, "privacyos/privacyos.db"); err != nil {
		tw.Close()
		gz.Close()
		os.Remove(target)
		return "", err
	}

	if err := tw.Close(); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}
	return target, nil
}

// appendFileToTar adds one file to an open tar stream.
func appendFileToTar(tw *tar.Writer, path, name string) error {
	if path == "" {
		return fmt.Errorf("数据库快照不可用")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = name
	header.Mode = 0o600
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(tw, f)
	return err
}

// RestoreBackup replaces the configuration from an uploaded archive.
func (s *Service) RestoreBackup(archivePath string) error {
	// Save the current configuration first.
	if _, err := s.CreateBackup(); err != nil {
		return err
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("不是有效的备份文件: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	foundDB := false
	var pending []struct {
		name string
		data []byte
	}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(hdr.Name, "privacyos/")
		rel = strings.TrimPrefix(rel, "/")
		if rel == hdr.Name || rel == "" || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			continue
		}
		if hdr.Typeflag != tar.TypeReg || hdr.Size > 512<<20 {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(tr, 512<<20))
		if err != nil {
			return err
		}
		if rel == "privacyos.db" {
			foundDB = true
		}
		pending = append(pending, struct {
			name string
			data []byte
		}{rel, data})
	}
	if !foundDB {
		return fmt.Errorf("备份中缺少 privacyos.db")
	}
	for _, p := range pending {
		target := filepath.Join(s.Cfg.ConfigDir, p.name)
		if !strings.HasPrefix(target, s.Cfg.ConfigDir+string(os.PathSeparator)) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(target, p.data, 0o600); err != nil {
			return err
		}
	}
	// Remove stale WAL/SHM so SQLite opens the restored database cleanly.
	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(filepath.Join(s.Cfg.ConfigDir, "privacyos.db"+suffix))
	}
	return nil
}

// GenerateSelfSigned creates a self-signed TLS certificate.
func (s *Service) GenerateSelfSigned(ctx context.Context, domain string) (string, string, error) {
	dir := filepath.Join(s.Cfg.ConfigDir, "tls")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{Organization: []string{"PrivacyOS"}, CommonName: firstNonEmpty(domain, "privacyos.local")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:     []string{"localhost", "privacyos.local"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	if domain != "" {
		tmpl.DNSNames = append(tmpl.DNSNames, domain)
	}
	for _, ip := range localIPs() {
		if parsed := net.ParseIP(ip); parsed != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, parsed)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return "", "", err
	}
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")
	certOut, err := os.OpenFile(certPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", "", err
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	certOut.Close()
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", "", err
	}
	keyOut, err := os.OpenFile(keyPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", "", err
	}
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	keyOut.Close()
	return certPath, keyPath, nil
}

// EncryptionInfo summarizes LUKS encryption state.
type EncryptionInfo struct {
	RootEncrypted bool          `json:"root_encrypted"`
	Volumes       []LuksVolume  `json:"volumes"`
	VaultCount    int           `json:"vault_count"`
}

// LuksVolume describes a LUKS device.
type LuksVolume struct {
	Device     string `json:"device"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Mountpoint string `json:"mountpoint,omitempty"`
}

// Encryption reports the host encryption state.
func (s *Service) Encryption(ctx context.Context) EncryptionInfo {
	info := EncryptionInfo{Volumes: []LuksVolume{}}
	if s.Dev() {
		info.RootEncrypted = s.Cfg.Pro()
		info.Volumes = []LuksVolume{{Device: "/dev/nvme0n1p3", Name: "nvme0n1p3_crypt", Type: "LUKS2", Status: "active", Mountpoint: "/"}}
		info.VaultCount = 1
		return info
	}
	out, err := s.run(ctx, 10*time.Second, "lsblk", "-J", "-o", "NAME,TYPE,FSTYPE,MOUNTPOINT")
	if err == nil {
		var doc struct {
			BlockDevices []struct {
				Name       string `json:"name"`
				Type       string `json:"type"`
				FSType     string `json:"fstype"`
				Mountpoint string `json:"mountpoint"`
				Children   []struct {
					Name       string `json:"name"`
					Type       string `json:"type"`
					FSType     string `json:"fstype"`
					Mountpoint string `json:"mountpoint"`
				} `json:"children"`
			} `json:"blockdevices"`
		}
		if json.Unmarshal([]byte(out), &doc) == nil {
			for _, d := range doc.BlockDevices {
				if d.FSType == "crypto_LUKS" {
					info.Volumes = append(info.Volumes, LuksVolume{
						Device: "/dev/" + d.Name, Name: d.Name, Type: "LUKS", Status: "inactive", Mountpoint: d.Mountpoint,
					})
				}
				for _, c := range d.Children {
					if c.FSType == "crypto_LUKS" || strings.HasPrefix(c.Type, "crypt") {
						info.Volumes = append(info.Volumes, LuksVolume{
							Device: "/dev/" + c.Name, Name: c.Name, Type: "LUKS2", Status: "active", Mountpoint: c.Mountpoint,
						})
					}
				}
			}
		}
	}
	if out, err := s.run(ctx, 5*time.Second, "findmnt", "-n", "-o", "SOURCE", "/"); err == nil {
		info.RootEncrypted = strings.HasPrefix(out, "/dev/mapper/")
	}
	vaults, _ := filepath.Glob(filepath.Join(s.Cfg.DataDir, "vaults", "*.luks"))
	info.VaultCount = len(vaults)
	return info
}

// ---------- Linux accounts ----------

// EnsureLinuxUser creates the Unix account backing a PrivacyOS user.
func (s *Service) EnsureLinuxUser(ctx context.Context, username, password string, pro bool) error {
	if s.Dev() {
		return nil
	}
	if !validUnixName(username) {
		return fmt.Errorf("非法用户名")
	}
	if _, err := user.Lookup(username); err != nil {
		home := filepath.Join(s.Cfg.DataDir, "users", username)
		mode := "0755"
		if pro {
			mode = "0700"
		}
		_, _ = s.run(ctx, 30*time.Second, "groupadd", "-f", "users")
		out, err := s.run(ctx, 30*time.Second, "useradd", "-M", "-d", home, "-s", "/usr/sbin/nologin", "-G", "users", username)
		if err != nil {
			return fmt.Errorf("useradd 失败: %v %s", err, out)
		}
		_ = os.MkdirAll(home, os.FileMode(mustParseMode(mode)))
	}
	if password != "" {
		if err := s.setPassword(ctx, username, password); err != nil {
			return err
		}
	}
	home := filepath.Join(s.Cfg.DataDir, "users", username)
	if usr, err := user.Lookup(username); err == nil {
		uid, _ := strconv.Atoi(usr.Uid)
		gid, _ := strconv.Atoi(usr.Gid)
		_ = os.Chown(home, uid, gid)
		perm := os.FileMode(0o755)
		if pro {
			perm = 0o700
		}
		_ = os.Chmod(home, perm)
	}
	return nil
}

// SetLinuxPassword updates the Unix and Samba password.
func (s *Service) SetLinuxPassword(ctx context.Context, username, password string) error {
	if s.Dev() {
		return nil
	}
	if !validUnixName(username) {
		return fmt.Errorf("非法用户名")
	}
	return s.setPassword(ctx, username, password)
}

func (s *Service) setPassword(ctx context.Context, username, password string) error {
	cmd := exec.CommandContext(ctx, "chpasswd")
	cmd.Stdin = strings.NewReader(username + ":" + password + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("设置 Linux 密码失败: %v %s", err, out)
	}
	return s.EnsureSambaUser(ctx, username, password)
}

// EnsureSambaUser syncs the Samba password database.
func (s *Service) EnsureSambaUser(ctx context.Context, username, password string) error {
	if s.Dev() {
		return nil
	}
	if _, err := exec.LookPath("smbpasswd"); err != nil {
		return nil
	}
	cmd := exec.CommandContext(ctx, "smbpasswd", "-a", "-s", username)
	cmd.Stdin = strings.NewReader(password + "\n" + password + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("同步 Samba 密码失败: %v %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// DeleteLinuxUser removes the Unix account but keeps home data.
func (s *Service) DeleteLinuxUser(ctx context.Context, username string) error {
	if s.Dev() {
		return nil
	}
	if !validUnixName(username) {
		return fmt.Errorf("非法用户名")
	}
	_, _ = s.run(ctx, 30*time.Second, "smbpasswd", "-x", username)
	out, err := s.run(ctx, 30*time.Second, "userdel", username)
	if err != nil {
		return fmt.Errorf("userdel 失败: %v %s", err, out)
	}
	return nil
}

// SetHomeMode tightens home permissions (0700 in the Pro edition).
func (s *Service) SetHomeMode(username string, pro bool) error {
	home := filepath.Join(s.Cfg.DataDir, "users", username)
	perm := os.FileMode(0o755)
	if pro {
		perm = 0o700
	}
	return os.Chmod(home, perm)
}

// ---------- helpers ----------

func osPrettyName() string {
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
			}
		}
	}
	return runtime.GOOS
}

func kernelVersion() string {
	if data, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		if v := strings.TrimSpace(string(data)); v != "" {
			return v
		}
	}
	out, err := exec.Command("uname", "-r").Output()
	if err == nil {
		if v := strings.TrimSpace(string(out)); v != "" {
			return v
		}
	}
	return runtime.GOOS
}

func cpuModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "model name") {
			if idx := strings.Index(line, ":"); idx > 0 {
				return strings.TrimSpace(line[idx+1:])
			}
		}
	}
	return ""
}

func localIPs() []string {
	var out []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return []string{"127.0.0.1"}
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil || ip4.IsLoopback() {
				continue
			}
			out = append(out, ip4.String())
		}
	}
	if len(out) == 0 {
		out = []string{"127.0.0.1"}
	}
	return out
}

func firstIP(ips []string) string {
	if len(ips) == 0 {
		return "127.0.0.1"
	}
	return ips[0]
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func mustParseMode(m string) int64 {
	v, _ := strconv.ParseInt(m, 8, 64)
	return v
}

func validUnixName(name string) bool {
	if name == "" || len(name) > 32 {
		return false
	}
	for i, r := range name {
		if i == 0 && (r >= '0' && r <= '9' || r == '-') {
			return false
		}
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

func (s *Service) run(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
