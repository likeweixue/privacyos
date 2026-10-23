// Package monitor collects live CPU, memory, network, disk and process stats.
package monitor

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"privacyos/backend/internal/config"
)

// Service samples host metrics.
type Service struct {
	Cfg *config.Config

	mu       sync.Mutex
	prevCPU  cpuTimes
	prevNet  map[string]netCounters
	prevDisk map[string]diskCounters
	prevProc map[int]procSample
	prevAt   time.Time
	history  []Snapshot
	samples  int
}

// New creates a monitor service.
func New(cfg *config.Config) *Service {
	return &Service{Cfg: cfg, prevNet: map[string]netCounters{}, prevDisk: map[string]diskCounters{}, prevProc: map[int]procSample{}}
}

// MemInfo describes memory usage.
type MemInfo struct {
	Total     int64   `json:"total"`
	Used      int64   `json:"used"`
	Available int64   `json:"available"`
	Percent   float64 `json:"percent"`
}

// NetInfo is the aggregate network throughput.
type NetInfo struct {
	RxBytes    int64   `json:"rx_bytes"`
	TxBytes    int64   `json:"tx_bytes"`
	RxRate     float64 `json:"rx_rate"`
	TxRate     float64 `json:"tx_rate"`
	Interfaces []NetInterface `json:"interfaces"`
}

// NetInterface is per-interface throughput.
type NetInterface struct {
	Name    string  `json:"name"`
	RxRate  float64 `json:"rx_rate"`
	TxRate  float64 `json:"tx_rate"`
	RxBytes int64   `json:"rx_bytes"`
	TxBytes int64   `json:"tx_bytes"`
}

// DiskIO is aggregate block IO.
type DiskIO struct {
	ReadRate  float64 `json:"read_rate"`
	WriteRate float64 `json:"write_rate"`
}

// Temp is a temperature sensor reading.
type Temp struct {
	Name  string  `json:"name"`
	Celsius float64 `json:"celsius"`
}

// DiskUsage is per-mount usage.
type DiskUsage struct {
	Mountpoint string  `json:"mountpoint"`
	Device     string  `json:"device,omitempty"`
	Total      int64   `json:"total"`
	Used       int64   `json:"used"`
	Percent    float64 `json:"percent"`
}

// Snapshot is a complete metrics sample.
type Snapshot struct {
	Time       string      `json:"time"`
	Hostname   string      `json:"hostname"`
	Version    string      `json:"-"`
	CPU        float64     `json:"cpu"`
	Cores      int         `json:"cores"`
	CPUModel   string      `json:"cpu_model,omitempty"`
	Load       [3]float64  `json:"load"`
	Mem        MemInfo     `json:"mem"`
	Swap       MemInfo     `json:"swap"`
	Net        NetInfo     `json:"net"`
	DiskIO     DiskIO      `json:"disk_io"`
	Uptime     int64       `json:"uptime"`
	Temps      []Temp      `json:"temps"`
	Disks      []DiskUsage `json:"disks"`
	Processes  int         `json:"processes"`
}

// Process is a process list row.
type Process struct {
	PID     int     `json:"pid"`
	Name    string  `json:"name"`
	User    string  `json:"user"`
	State   string  `json:"state"`
	CPU     float64 `json:"cpu"`
	Mem     int64   `json:"mem"`
	Threads int     `json:"threads"`
}

// Snapshot returns the latest metrics sample.
func (s *Service) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Cfg.Dev || runtime.GOOS != "linux" {
		return s.devSnapshotLocked()
	}
	snap := s.readLinuxLocked()
	s.history = append(s.history, snap)
	if len(s.history) > 300 {
		s.history = s.history[len(s.history)-300:]
	}
	return snap
}

// History returns recent samples.
func (s *Service) History(limit int) []Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || limit > len(s.history) {
		limit = len(s.history)
	}
	out := make([]Snapshot, limit)
	copy(out, s.history[len(s.history)-limit:])
	return out
}

type cpuTimes struct {
	total, idle uint64
}

type netCounters struct {
	rx, tx int64
}

type diskCounters struct {
	read, write int64
}

type procSample struct {
	ticks uint64
	at    time.Time
}

func (s *Service) readLinuxLocked() Snapshot {
	now := time.Now()
	snap := Snapshot{Time: now.UTC().Format(time.RFC3339), Cores: runtime.NumCPU()}
	snap.Hostname, _ = os.Hostname()

	// CPU
	if ct, err := readCPUTimes(); err == nil {
		if s.prevCPU.total > 0 && ct.total > s.prevCPU.total {
			dt := float64(ct.total - s.prevCPU.total)
			di := float64(ct.idle - s.prevCPU.idle)
			snap.CPU = round1((dt - di) / dt * 100)
		}
		s.prevCPU = ct
	}
	snap.CPUModel = cpuModel()

	// Load / uptime
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		var l [3]float64
		fmt.Sscanf(string(data), "%f %f %f", &l[0], &l[1], &l[2])
		snap.Load = l
	}
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		var up float64
		fmt.Sscanf(string(data), "%f", &up)
		snap.Uptime = int64(up)
	}

	// Memory
	snap.Mem = readMemInfo()
	snap.Swap = readSwap()

	// Network
	elapsed := now.Sub(s.prevAt).Seconds()
	if elapsed <= 0 || elapsed > 30 {
		elapsed = 2
	}
	snap.Net = s.readNet(elapsed)
	snap.DiskIO = s.readDiskIO(elapsed)
	s.prevAt = now

	snap.Temps = readTemps()
	snap.Disks = readDiskUsage()

	// Process count
	if entries, err := os.ReadDir("/proc"); err == nil {
		for _, e := range entries {
			if _, err := strconv.Atoi(e.Name()); err == nil {
				snap.Processes++
			}
		}
	}
	if snap.Processes == 0 {
		snap.Processes = runtime.NumGoroutine()
	}
	return snap
}

func readCPUTimes() (cpuTimes, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuTimes{}, err
	}
	line := strings.SplitN(string(data), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return cpuTimes{}, fmt.Errorf("bad /proc/stat")
	}
	var total, idle uint64
	for i, f := range fields[1:] {
		v, _ := strconv.ParseUint(f, 10, 64)
		total += v
		if i == 3 || i == 4 {
			idle += v
		}
	}
	return cpuTimes{total: total, idle: idle}, nil
}

func readMemInfo() MemInfo {
	var total, avail int64
	readKeyValues("/proc/meminfo", func(k string, v int64) {
		switch k {
		case "MemTotal":
			total = v * 1024
		case "MemAvailable":
			avail = v * 1024
		}
	})
	used := total - avail
	return MemInfo{Total: total, Used: used, Available: avail, Percent: percent(used, total)}
}

func readSwap() MemInfo {
	var total, free int64
	readKeyValues("/proc/meminfo", func(k string, v int64) {
		switch k {
		case "SwapTotal":
			total = v * 1024
		case "SwapFree":
			free = v * 1024
		}
	})
	used := total - free
	return MemInfo{Total: total, Used: used, Available: free, Percent: percent(used, total)}
}

func readKeyValues(path string, fn func(string, int64)) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		v, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		fn(strings.TrimSuffix(fields[0], ":"), v)
	}
}

func (s *Service) readNet(elapsed float64) NetInfo {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return NetInfo{}
	}
	var info NetInfo
	for i, line := range strings.Split(string(data), "\n") {
		if i < 2 || !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		name := strings.TrimSpace(parts[0])
		if name == "lo" {
			continue
		}
		f := strings.Fields(parts[1])
		if len(f) < 9 {
			continue
		}
		rx, _ := strconv.ParseInt(f[0], 10, 64)
		tx, _ := strconv.ParseInt(f[8], 10, 64)
		prev := s.prevNet[name]
		iface := NetInterface{Name: name, RxBytes: rx, TxBytes: tx}
		if prev.rx > 0 {
			iface.RxRate = round1(float64(rx-prev.rx) / elapsed)
			iface.TxRate = round1(float64(tx-prev.tx) / elapsed)
		}
		info.RxBytes += rx
		info.TxBytes += tx
		info.RxRate += iface.RxRate
		info.TxRate += iface.TxRate
		info.Interfaces = append(info.Interfaces, iface)
		s.prevNet[name] = netCounters{rx: rx, tx: tx}
	}
	sort.Slice(info.Interfaces, func(a, b int) bool {
		return info.Interfaces[a].RxRate+info.Interfaces[a].TxRate > info.Interfaces[b].RxRate+info.Interfaces[b].TxRate
	})
	return info
}

func (s *Service) readDiskIO(elapsed float64) DiskIO {
	data, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		return DiskIO{}
	}
	var io DiskIO
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) < 14 {
			continue
		}
		name := f[2]
		if !isPhysicalDisk(name) {
			continue
		}
		read, _ := strconv.ParseInt(f[5], 10, 64)
		write, _ := strconv.ParseInt(f[9], 10, 64)
		prev := s.prevDisk[name]
		if prev.read > 0 {
			io.ReadRate += float64(read-prev.read) * 512 / elapsed
			io.WriteRate += float64(write-prev.write) * 512 / elapsed
		}
		s.prevDisk[name] = diskCounters{read: read, write: write}
	}
	io.ReadRate = round1(io.ReadRate)
	io.WriteRate = round1(io.WriteRate)
	return io
}

func isPhysicalDisk(name string) bool {
	if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") ||
		strings.HasPrefix(name, "dm-") || strings.HasPrefix(name, "md") {
		return false
	}
	if strings.HasPrefix(name, "sd") || strings.HasPrefix(name, "nvme") ||
		strings.HasPrefix(name, "vd") || strings.HasPrefix(name, "hd") {
		// skip partitions: sda1, nvme0n1p2
		if strings.HasPrefix(name, "nvme") {
			return !strings.Contains(name, "p")
		}
		last := name[len(name)-1]
		return last < '0' || last > '9'
	}
	return false
}

func readTemps() []Temp {
	var out []Temp
	hwmons, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	for _, dir := range hwmons {
		nameBytes, _ := os.ReadFile(filepath.Join(dir, "name"))
		name := strings.TrimSpace(string(nameBytes))
		inputs, _ := filepath.Glob(filepath.Join(dir, "temp*_input"))
		for _, in := range inputs {
			raw, err := os.ReadFile(in)
			if err != nil {
				continue
			}
			milli, err := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
			if err != nil || milli <= 0 {
				continue
			}
			out = append(out, Temp{Name: name, Celsius: round1(milli / 1000)})
		}
	}
	if len(out) == 0 {
		zones, _ := filepath.Glob("/sys/class/thermal/thermal_zone*")
		for i, z := range zones {
			raw, err := os.ReadFile(filepath.Join(z, "temp"))
			if err != nil {
				continue
			}
			milli, err := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
			if err != nil {
				continue
			}
			out = append(out, Temp{Name: fmt.Sprintf("thermal%d", i), Celsius: round1(milli / 1000)})
		}
	}
	return out
}

func readDiskUsage() []DiskUsage {
	paths := []string{"/", "/data"}
	var out []DiskUsage
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		var st syscall.Statfs_t
		if err := syscall.Statfs(p, &st); err != nil {
			continue
		}
		total := st.Blocks * uint64(st.Bsize)
		avail := st.Bavail * uint64(st.Bsize)
		if total == 0 {
			continue
		}
		used := total - avail
		out = append(out, DiskUsage{Mountpoint: p, Total: int64(total), Used: int64(used), Percent: percent(int64(used), int64(total))})
	}
	return out
}

func cpuModel() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "model name") || strings.HasPrefix(line, "Hardware") {
			if idx := strings.Index(line, ":"); idx > 0 {
				return strings.TrimSpace(line[idx+1:])
			}
		}
	}
	return ""
}

// Processes returns the process list sorted by CPU usage.
func (s *Service) Processes() ([]Process, error) {
	if s.Cfg.Dev || runtime.GOOS != "linux" {
		return demoProcesses(), nil
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var out []Process
	newSamples := map[int]procSample{}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		p, ticks, err := readProcess(pid)
		if err != nil {
			continue
		}
		if prev, ok := s.prevProc[pid]; ok {
			elapsed := now.Sub(prev.at).Seconds()
			if elapsed > 0 && ticks >= prev.ticks {
				p.CPU = round1(float64(ticks-prev.ticks) / elapsed)
			}
		}
		newSamples[pid] = procSample{ticks: ticks, at: now}
		out = append(out, p)
	}
	s.prevProc = newSamples
	sort.Slice(out, func(a, b int) bool { return out[a].CPU > out[b].CPU })
	if len(out) > 200 {
		out = out[:200]
	}
	return out, nil
}

func readProcess(pid int) (Process, uint64, error) {
	statData, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return Process{}, 0, err
	}
	raw := string(statData)
	l := strings.Index(raw, "(")
	r := strings.LastIndex(raw, ")")
	if l < 0 || r < 0 {
		return Process{}, 0, fmt.Errorf("bad stat")
	}
	name := raw[l+1 : r]
	rest := strings.Fields(raw[r+2:])
	if len(rest) < 22 {
		return Process{}, 0, fmt.Errorf("bad stat")
	}
	utime, _ := strconv.ParseUint(rest[11], 10, 64)
	stime, _ := strconv.ParseUint(rest[12], 10, 64)
	rss, _ := strconv.ParseUint(rest[21], 10, 64)
	state := rest[0]
	threads, _ := strconv.Atoi(rest[17])
	var uid string
	if status, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid)); err == nil {
		for _, line := range strings.Split(string(status), "\n") {
			if strings.HasPrefix(line, "Uid:") {
				f := strings.Fields(line)
				if len(f) > 1 {
					uid = f[1]
				}
				break
			}
		}
	}
	return Process{
		PID: pid, Name: name, User: uid, State: state,
		Mem: int64(rss) * 4096, Threads: threads,
	}, utime + stime, nil
}

// Kill terminates a process after validation.
func (s *Service) Kill(pid int, force bool) error {
	if pid <= 1 {
		return fmt.Errorf("拒绝结束系统进程")
	}
	if pid == os.Getpid() {
		return fmt.Errorf("拒绝结束 PrivacyOS 自身")
	}
	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}
	return syscall.Kill(pid, sig)
}

// ---------- dev data ----------

func (s *Service) devSnapshotLocked() Snapshot {
	s.samples++
	t := float64(s.samples)
	snap := Snapshot{
		Time: time.Now().UTC().Format(time.RFC3339), Hostname: "privacyos-dev",
		Cores: 8, CPUModel: "Apple M2 (开发模式模拟)",
		CPU: round1(18 + 9*math.Sin(t/7) + 4*math.Sin(t/2.3)),
		Load: [3]float64{0.42, 0.38, 0.31},
		Uptime: 3*86400 + int64(t*2),
		Processes: 187,
	}
	snap.Mem = MemInfo{Total: 16 * 1024 * 1024 * 1024, Used: 6_800_000_000, Available: 9_200_000_000}
	snap.Mem.Percent = percent(snap.Mem.Used, snap.Mem.Total)
	snap.Swap = MemInfo{Total: 4 * 1024 * 1024 * 1024, Used: 220_000_000}
	snap.Swap.Percent = percent(snap.Swap.Used, snap.Swap.Total)
	snap.Net = NetInfo{
		RxRate:   round1(1_800_000 + 900_000*math.Sin(t/3.1)),
		TxRate:   round1(420_000 + 260_000*math.Sin(t/4.2)),
		RxBytes: 128_000_000_000, TxBytes: 21_000_000_000,
		Interfaces: []NetInterface{
			{Name: "eth0", RxRate: 1_600_000, TxRate: 380_000, RxBytes: 121_000_000_000, TxBytes: 19_000_000_000},
			{Name: "wlan0", RxRate: 120_000, TxRate: 40_000, RxBytes: 7_000_000_000, TxBytes: 2_000_000_000},
		},
	}
	snap.DiskIO = DiskIO{ReadRate: round1(2_400_000 + 1_200_000*math.Sin(t/5)), WriteRate: round1(900_000 + 500_000*math.Sin(t/3.7))}
	snap.Temps = []Temp{{Name: "cpu", Celsius: round1(38 + 4*math.Sin(t/9))}, {Name: "nvme", Celsius: round1(41 + 2*math.Sin(t/11))}}
	snap.Disks = []DiskUsage{
		{Mountpoint: "/", Device: "/dev/nvme0n1p2", Total: 118_111_600_640, Used: 41_231_686_041, Percent: 34.9},
		{Mountpoint: "/data", Device: "/dev/md0", Total: 7_998_947_098_624, Used: 2_319_282_339_840, Percent: 29.0},
	}
	s.history = append(s.history, snap)
	if len(s.history) > 300 {
		s.history = s.history[len(s.history)-300:]
	}
	return snap
}

func demoProcesses() []Process {
	return []Process{
		{PID: 1234, Name: "privacyos-server", User: "root", State: "S", CPU: 1.8, Mem: 86_000_000, Threads: 14},
		{PID: 812, Name: "dockerd", User: "root", State: "S", CPU: 1.2, Mem: 142_000_000, Threads: 32},
		{PID: 1543, Name: "smbd", User: "root", State: "S", CPU: 0.7, Mem: 48_000_000, Threads: 3},
		{PID: 2201, Name: "ffmpeg", User: "demo", State: "R", CPU: 0.4, Mem: 210_000_000, Threads: 6},
		{PID: 98, Name: "systemd-journald", User: "root", State: "S", CPU: 0.2, Mem: 28_000_000, Threads: 2},
		{PID: 415, Name: "nginx", User: "www-data", State: "S", CPU: 0.1, Mem: 14_000_000, Threads: 2},
	}
}

func percent(used, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return round1(float64(used) / float64(total) * 100)
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
