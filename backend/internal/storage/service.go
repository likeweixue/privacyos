// Package storage manages disks, SMART health and software RAID pools.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"privacyos/backend/internal/config"
	"privacyos/backend/internal/store"
)

// Disk describes a physical block device.
type Disk struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Model       string `json:"model"`
	Serial      string `json:"serial"`
	SizeBytes   int64  `json:"size_bytes"`
	Type        string `json:"type"`
	Rotational  bool   `json:"rotational"`
	FSType      string `json:"fstype,omitempty"`
	Mountpoint  string `json:"mountpoint,omitempty"`
	Health      string `json:"health"` // passed | failing | unknown | unavailable
	Temperature int    `json:"temperature,omitempty"`
	UsedBytes   int64  `json:"used_bytes,omitempty"`
	UsedPercent float64 `json:"used_percent,omitempty"`
	PowerOnHours int64 `json:"power_on_hours,omitempty"`
}

// Pool is a RAID/filesystem pool with live status.
type Pool struct {
	store.StoragePool
	Active   bool   `json:"active"`
	Member   string `json:"member_state,omitempty"`
	UsedBytes int64 `json:"used_bytes,omitempty"`
	UsedPercent float64 `json:"used_percent,omitempty"`
}

// Service provides storage operations.
type Service struct {
	Cfg   *config.Config
	Store *store.Store
}

// New creates a storage service.
func New(cfg *config.Config, st *store.Store) *Service {
	return &Service{Cfg: cfg, Store: st}
}

// Dev reports whether hardware access is simulated.
func (s *Service) Dev() bool { return s.Cfg.Dev || runtime.GOOS != "linux" }

type lsblkDevice struct {
	Name       string        `json:"name"`
	Path       string        `json:"path"`
	Model      string        `json:"model"`
	Serial     string        `json:"serial"`
	Size       int64         `json:"size"`
	Rotational json.RawMessage `json:"rota"`
	Type       string        `json:"type"`
	FSType     string        `json:"fstype"`
	Mountpoint string        `json:"mountpoint"`
	Children   []lsblkDevice `json:"children"`
}

type lsblkOutput struct {
	BlockDevices []lsblkDevice `json:"blockdevices"`
}

// ListDisks returns physical disks with SMART health.
func (s *Service) ListDisks(ctx context.Context) ([]Disk, error) {
	if s.Dev() {
		return demoDisks(), nil
	}
	out, err := s.run(ctx, 10*time.Second, "lsblk", "-J", "-b", "-o",
		"NAME,PATH,MODEL,SERIAL,SIZE,ROTA,TYPE,FSTYPE,MOUNTPOINT")
	if err != nil {
		return nil, fmt.Errorf("lsblk: %w", err)
	}
	var parsed lsblkOutput
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return nil, err
	}
	var disks []Disk
	for _, d := range parsed.BlockDevices {
		if d.Type != "disk" {
			continue
		}
		disk := Disk{
			Name: d.Name, Path: d.Path, Model: strings.TrimSpace(d.Model),
			Serial: strings.TrimSpace(d.Serial), SizeBytes: d.Size,
			Type: d.Type, FSType: d.FSType, Mountpoint: d.Mountpoint,
		}
		disk.Rotational = strings.Trim(string(d.Rotational), `"`) == "1" || string(d.Rotational) == "1"
		for _, c := range d.Children {
			if disk.FSType == "" && c.FSType != "" {
				disk.FSType = c.FSType
			}
			if disk.Mountpoint == "" && c.Mountpoint != "" {
				disk.Mountpoint = c.Mountpoint
			}
		}
		s.fillSMART(ctx, &disk)
		if disk.Mountpoint != "" {
			if used, pct, err := Usage(disk.Mountpoint); err == nil {
				disk.UsedBytes, disk.UsedPercent = used, pct
			}
		}
		disks = append(disks, disk)
	}
	return disks, nil
}

// fillSMART enriches a disk with smartctl data when available.
func (s *Service) fillSMART(ctx context.Context, d *Disk) {
	d.Health = "unavailable"
	if _, err := exec.LookPath("smartctl"); err != nil {
		return
	}
	out, err := s.run(ctx, 12*time.Second, "smartctl", "-a", "-j", d.Path)
	if err != nil && out == "" {
		return
	}
	var doc smartDoc
	if json.Unmarshal([]byte(out), &doc) != nil {
		d.Health = "unknown"
		return
	}
	switch {
	case doc.SmartStatus.Passed:
		d.Health = "passed"
	case doc.SmartStatus.Passed == false && doc.SmartStatus.String != "":
		d.Health = "failing"
	default:
		d.Health = "unknown"
	}
	if doc.Temperature.Current > 0 {
		d.Temperature = doc.Temperature.Current
	}
	d.PowerOnHours = doc.PowerOnTime.Hours
}

type smartDoc struct {
	SmartStatus struct {
		Passed bool   `json:"passed"`
		String string `json:"string"`
	} `json:"smart_status"`
	Temperature struct {
		Current int `json:"current"`
	} `json:"temperature"`
	PowerOnTime struct {
		Hours int64 `json:"hours"`
	} `json:"power_on_time"`
}

// Smart returns raw SMART data for a device.
func (s *Service) Smart(ctx context.Context, device string) (map[string]any, error) {
	if s.Dev() {
		return map[string]any{
			"device": device, "model": "WDC WD40EFPX-68C6CN0", "serial": "WD-DEMO-0001",
			"smart_status": map[string]any{"passed": true},
			"temperature":  map[string]any{"current": 34},
			"power_on_time": map[string]any{"hours": 12043},
			"ata_smart_attributes": map[string]any{
				"table": []map[string]any{
					{"id": 5, "name": "Reallocated_Sector_Ct", "value": 100, "worst": 100, "thresh": 10, "raw": map[string]any{"value": 0}},
					{"id": 197, "name": "Current_Pending_Sector", "value": 100, "worst": 100, "thresh": 0, "raw": map[string]any{"value": 0}},
					{"id": 194, "name": "Temperature_Celsius", "value": 66, "worst": 45, "thresh": 0, "raw": map[string]any{"value": 34}},
				},
			},
			"demo": true,
		}, nil
	}
	if !validDevice(device) {
		return nil, fmt.Errorf("非法设备名")
	}
	out, err := s.run(ctx, 20*time.Second, "smartctl", "-a", "-j", device)
	if err != nil && out == "" {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		return map[string]any{"raw": out}, nil
	}
	return doc, nil
}

// RunSmartTest starts a SMART self-test and records it.
func (s *Service) RunSmartTest(ctx context.Context, device, kind string) (*store.SmartTest, error) {
	if kind != "short" && kind != "long" && kind != "conveyance" {
		kind = "short"
	}
	summary := "已下发 SMART " + kind + " 测试"
	if s.Dev() {
		summary = "开发模式：模拟 SMART " + kind + " 测试通过"
	} else {
		if !validDevice(device) {
			return nil, fmt.Errorf("非法设备名")
		}
		out, err := s.run(ctx, 30*time.Second, "smartctl", "-t", kind, device)
		if err != nil {
			return nil, fmt.Errorf("%v: %s", err, out)
		}
	}
	test := &store.SmartTest{Device: device, Kind: kind, Status: "completed", Summary: summary}
	if err := s.Store.AddSmartTest(test); err != nil {
		return nil, err
	}
	return test, nil
}

// ListPools returns configured pools merged with live MD status.
func (s *Service) ListPools() ([]Pool, error) {
	rows, err := s.Store.ListPools()
	if err != nil {
		return nil, err
	}
	md := parseMDStat()
	out := make([]Pool, 0, len(rows))
	for _, p := range rows {
		pool := Pool{StoragePool: p}
		if info, ok := md["/dev/"+p.Name]; ok {
			pool.Active = info.active
			pool.Member = info.members
		}
		pool.Active = pool.Active || p.Status == "active"
		if p.Mountpoint != "" {
			if used, pct, err := Usage(p.Mountpoint); err == nil {
				pool.UsedBytes, pool.UsedPercent = used, pct
			}
		}
		out = append(out, pool)
	}
	return out, nil
}

type mdInfo struct {
	active  bool
	members string
}

func parseMDStat() map[string]mdInfo {
	out := map[string]mdInfo{}
	data, err := os.ReadFile("/proc/mdstat")
	if err != nil {
		return out
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "md") {
			continue
		}
		fields := strings.SplitN(line, ":", 2)
		if len(fields) != 2 {
			continue
		}
		dev := "/dev/" + strings.TrimSpace(fields[0])
		out[dev] = mdInfo{active: strings.Contains(fields[1], "active"), members: strings.TrimSpace(fields[1])}
	}
	return out
}

// CreatePool creates an MD RAID array, formats and mounts it.
func (s *Service) CreatePool(ctx context.Context, name, level, fs string, devices []string, mountpoint string) (*store.StoragePool, error) {
	if name == "" || strings.ContainsAny(name, "/ ") {
		return nil, fmt.Errorf("非法存储池名称")
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("至少需要一块磁盘")
	}
	for _, d := range devices {
		if !validDevice(d) {
			return nil, fmt.Errorf("非法设备名 %q", d)
		}
	}
	if fs == "" {
		fs = "ext4"
	}
	if fs != "ext4" && fs != "xfs" && fs != "btrfs" {
		return nil, fmt.Errorf("不支持的文件系统 %q", fs)
	}
	if mountpoint == "" {
		mountpoint = filepath.Join("/mnt", name)
	}
	pool := &store.StoragePool{
		Name: name, Level: level, FS: fs, Devices: devices,
		Mountpoint: mountpoint, Status: "creating",
	}
	if s.Dev() {
		pool.Status = "active"
		pool.SizeBytes = int64(len(devices)) * 4 * 1024 * 1024 * 1024
		if err := s.Store.UpsertPool(pool); err != nil {
			return nil, err
		}
		return &store.StoragePool{ID: pool.ID, Name: name, Level: level, FS: fs, Devices: devices,
			Mountpoint: mountpoint, Status: "active", SizeBytes: pool.SizeBytes,
			CreatedAt: pool.CreatedAt, UpdatedAt: pool.UpdatedAt}, nil
	}

	args := []string{"--create", "/dev/md/" + name, "--level=" + level, "--raid-devices=" + strconv.Itoa(len(devices)), "--force", "--run"}
	args = append(args, devices...)
	if _, err := s.run(ctx, 120*time.Second, "mdadm", args...); err != nil {
		return nil, fmt.Errorf("mdadm 创建失败: %w", err)
	}
	mdDev := "/dev/md/" + name
	if _, err := s.run(ctx, 120*time.Second, "mkfs."+fs, "-F", mdDev); err != nil {
		return nil, fmt.Errorf("格式化失败: %w", err)
	}
	if err := os.MkdirAll(mountpoint, 0o750); err != nil {
		return nil, err
	}
	if _, err := s.run(ctx, 30*time.Second, "mount", mdDev, mountpoint); err != nil {
		return nil, fmt.Errorf("挂载失败: %w", err)
	}
	pool.Status = "active"
	if size, _, err := Usage(mountpoint); err == nil {
		pool.SizeBytes = size
	}
	if err := s.Store.UpsertPool(pool); err != nil {
		return nil, err
	}
	return pool, nil
}

// MountPool mounts a pool.
func (s *Service) MountPool(ctx context.Context, id string) error {
	p, err := s.Store.GetPool(id)
	if err != nil {
		return err
	}
	if s.Dev() {
		p.Status = "active"
		return s.Store.UpsertPool(p)
	}
	if _, err := s.run(ctx, 30*time.Second, "mount", "/dev/md/"+p.Name, p.Mountpoint); err != nil {
		return err
	}
	p.Status = "active"
	return s.Store.UpsertPool(p)
}

// UnmountPool unmounts a pool.
func (s *Service) UnmountPool(ctx context.Context, id string) error {
	p, err := s.Store.GetPool(id)
	if err != nil {
		return err
	}
	if s.Dev() {
		p.Status = "inactive"
		return s.Store.UpsertPool(p)
	}
	if _, err := s.run(ctx, 30*time.Second, "umount", p.Mountpoint); err != nil {
		return err
	}
	p.Status = "inactive"
	return s.Store.UpsertPool(p)
}

// DeletePool unmounts and stops a pool, optionally wiping superblocks.
func (s *Service) DeletePool(ctx context.Context, id string, wipe bool) error {
	p, err := s.Store.GetPool(id)
	if err != nil {
		return err
	}
	if !s.Dev() {
		_, _ = s.run(ctx, 20*time.Second, "umount", p.Mountpoint)
		_, _ = s.run(ctx, 30*time.Second, "mdadm", "--stop", "/dev/md/"+p.Name)
		if wipe {
			for _, dev := range p.Devices {
				if validDevice(dev) {
					_, _ = s.run(ctx, 20*time.Second, "wipefs", "-a", dev)
				}
			}
		}
	}
	return s.Store.DeletePool(id)
}

// ExtendPool adds devices to an array.
func (s *Service) ExtendPool(ctx context.Context, id string, devices []string) error {
	p, err := s.Store.GetPool(id)
	if err != nil {
		return err
	}
	for _, d := range devices {
		if !validDevice(d) {
			return fmt.Errorf("非法设备名 %q", d)
		}
	}
	if !s.Dev() {
		for _, d := range devices {
			if _, err := s.run(ctx, 30*time.Second, "mdadm", "--add", "/dev/md/"+p.Name, d); err != nil {
				return err
			}
		}
		_, _ = s.run(ctx, 30*time.Second, "mdadm", "--grow", "/dev/md/"+p.Name, "--raid-devices="+strconv.Itoa(len(p.Devices)+len(devices)))
		_, _ = s.run(ctx, 120*time.Second, "resize2fs", "/dev/md/"+p.Name)
	}
	p.Devices = append(p.Devices, devices...)
	return s.Store.UpsertPool(p)
}

// Usage returns used bytes and percent for a mountpoint.
func Usage(path string) (int64, float64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	total := st.Blocks * uint64(st.Bsize)
	free := st.Bavail * uint64(st.Bsize)
	used := total - free
	pct := 0.0
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}
	return int64(used), pct, nil
}

// Mounts lists mounted filesystems and their usage.
func (s *Service) Mounts(ctx context.Context) ([]map[string]any, error) {
	if s.Dev() {
		return []map[string]any{
			{"mountpoint": "/", "device": "/dev/nvme0n1p2", "fstype": "ext4", "size": 118111600640, "used": 41231686041, "percent": 34.9},
			{"mountpoint": "/data", "device": "/dev/md0", "fstype": "ext4", "size": 7998947098624, "used": 2319282339840, "percent": 29.0},
			{"mountpoint": "/data/backups", "device": "/dev/sdb1", "fstype": "xfs", "size": 4000787030016, "used": 812345678912, "percent": 20.3},
		}, nil
	}
	out, err := s.run(ctx, 10*time.Second, "df", "-B1", "-T", "-x", "tmpfs", "-x", "devtmpfs", "-x", "squashfs")
	if err != nil {
		return nil, err
	}
	var mounts []map[string]any
	for i, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if i == 0 {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 7 {
			continue
		}
		size, _ := strconv.ParseInt(f[2], 10, 64)
		used, _ := strconv.ParseInt(f[3], 10, 64)
		pct, _ := strconv.ParseFloat(strings.TrimSuffix(f[5], "%"), 64)
		mounts = append(mounts, map[string]any{
			"device": f[0], "fstype": f[1], "size": size, "used": used,
			"percent": pct, "mountpoint": strings.Join(f[6:], " "),
		})
	}
	return mounts, nil
}

func (s *Service) run(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func validDevice(dev string) bool {
	if !strings.HasPrefix(dev, "/dev/") || strings.Contains(dev, "..") {
		return false
	}
	for _, r := range dev {
		if !(r == '/' || r == '-' || r == '_' || r == '.' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

func demoDisks() []Disk {
	return []Disk{
		{Name: "nvme0n1", Path: "/dev/nvme0n1", Model: "Samsung SSD 990 PRO 1TB", Serial: "S6Z1NJ0W123456",
			SizeBytes: 1000204886016, Type: "disk", FSType: "ext4", Mountpoint: "/",
			Health: "passed", Temperature: 38, UsedBytes: 41231686041, UsedPercent: 4.1, PowerOnHours: 3120},
		{Name: "sda", Path: "/dev/sda", Model: "WDC WD40EFPX-68C6CN0", Serial: "WD-WCC4N1234567",
			SizeBytes: 4000787030016, Type: "disk", Rotational: true, FSType: "linux_raid_member", Mountpoint: "",
			Health: "passed", Temperature: 34, PowerOnHours: 12043},
		{Name: "sdb", Path: "/dev/sdb", Model: "WDC WD40EFPX-68C6CN0", Serial: "WD-WCC4N7654321",
			SizeBytes: 4000787030016, Type: "disk", Rotational: true, FSType: "linux_raid_member", Mountpoint: "",
			Health: "failing", Temperature: 41, PowerOnHours: 21077},
		{Name: "sdc", Path: "/dev/sdc", Model: "Seagate IronWolf ST8000VN004", Serial: "ZA12ABCD",
			SizeBytes: 8001563222016, Type: "disk", Rotational: true, FSType: "ext4", Mountpoint: "/data/backups",
			Health: "passed", Temperature: 36, UsedBytes: 812345678912, UsedPercent: 10.3, PowerOnHours: 6402},
	}
}
