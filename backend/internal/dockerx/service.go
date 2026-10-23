// Package dockerx talks to the Docker Engine API over its unix socket and
// manages docker-compose projects.
package dockerx

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"privacyos/backend/internal/config"
)

// Service wraps the Docker Engine API.
type Service struct {
	Cfg        *config.Config
	socket     string
	client     *http.Client
	composeDir string
}

// New creates a Docker service.
func New(cfg *config.Config) *Service {
	socket := os.Getenv("DOCKER_HOST")
	if socket == "" {
		socket = "/var/run/docker.sock"
	}
	socket = strings.TrimPrefix(socket, "unix://")
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", socket)
		},
	}
	return &Service{
		Cfg:    cfg,
		socket: socket,
		client: &http.Client{Transport: transport, Timeout: 30 * time.Second},
		composeDir: filepath.Join(cfg.DataDir, "docker", "compose"),
	}
}

// Dev reports whether Docker is simulated.
func (s *Service) Dev() bool { return s.Cfg.Dev }

// Port is a published container port.
type Port struct {
	Private int    `json:"private"`
	Public  int    `json:"public"`
	Type    string `json:"type"`
	IP      string `json:"ip,omitempty"`
}

// Container is a container list row.
type Container struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	State   string `json:"state"`
	Status  string `json:"status"`
	Ports   []Port `json:"ports"`
	Created int64  `json:"created"`
	CPU     float64 `json:"cpu,omitempty"`
	Memory  int64  `json:"memory,omitempty"`
}

// Image is an image list row.
type Image struct {
	ID      string   `json:"id"`
	Tags    []string `json:"tags"`
	Size    int64    `json:"size"`
	Created int64    `json:"created"`
}

// Stats is a point-in-time container resource sample.
type Stats struct {
	CPU         float64 `json:"cpu"`
	MemUsage    int64   `json:"mem_usage"`
	MemLimit    int64   `json:"mem_limit"`
	NetRx       int64   `json:"net_rx"`
	NetTx       int64   `json:"net_tx"`
	BlockRead   int64   `json:"block_read"`
	BlockWrite  int64   `json:"block_write"`
}

// Status reports Docker availability and version.
func (s *Service) Status(ctx context.Context) map[string]any {
	if s.Dev() {
		return map[string]any{"available": true, "version": "27.3.1", "api_version": "1.47", "socket": "模拟 (开发模式)", "demo": true}
	}
	var ver struct {
		Version    string `json:"Version"`
		APIVersion string `json:"ApiVersion"`
	}
	if err := s.getJSON(ctx, "/version", &ver); err != nil {
		return map[string]any{"available": false, "error": err.Error(), "socket": s.socket}
	}
	return map[string]any{"available": true, "version": ver.Version, "api_version": ver.APIVersion, "socket": s.socket}
}

// ListContainers returns containers from the engine.
func (s *Service) ListContainers(ctx context.Context, all bool) ([]Container, error) {
	if s.Dev() {
		return demoContainers(), nil
	}
	path := "/containers/json?all=1"
	if !all {
		path = "/containers/json"
	}
	var raw []struct {
		ID      string `json:"Id"`
		Names   []string `json:"Names"`
		Image   string `json:"Image"`
		State   string `json:"State"`
		Status  string `json:"Status"`
		Created int64  `json:"Created"`
		Ports   []struct {
			PrivatePort int    `json:"PrivatePort"`
			PublicPort  int    `json:"PublicPort"`
			Type        string `json:"Type"`
			IP          string `json:"IP"`
		} `json:"Ports"`
	}
	if err := s.getJSON(ctx, path, &raw); err != nil {
		return nil, err
	}
	out := make([]Container, 0, len(raw))
	for _, c := range raw {
		name := strings.TrimPrefix(strings.Join(c.Names, ","), "/")
		item := Container{ID: c.ID, Name: name, Image: c.Image, State: c.State, Status: c.Status, Created: c.Created}
		for _, p := range c.Ports {
			item.Ports = append(item.Ports, Port{Private: p.PrivatePort, Public: p.PublicPort, Type: p.Type, IP: p.IP})
		}
		out = append(out, item)
	}
	return out, nil
}

// Action starts/stops/restarts/kills a container.
func (s *Service) Action(ctx context.Context, id, action string) error {
	if !validRef(id) {
		return fmt.Errorf("非法容器 ID")
	}
	switch action {
	case "start", "stop", "restart", "kill", "pause", "unpause":
	default:
		return fmt.Errorf("不支持的操作 %q", action)
	}
	if s.Dev() {
		return nil
	}
	_, err := s.do(ctx, http.MethodPost, "/containers/"+id+"/"+action, nil)
	return err
}

// Remove deletes a container.
func (s *Service) Remove(ctx context.Context, id string, force bool) error {
	if !validRef(id) {
		return fmt.Errorf("非法容器 ID")
	}
	if s.Dev() {
		return nil
	}
	path := "/containers/" + id + "?v=1"
	if force {
		path += "&force=1"
	}
	_, err := s.do(ctx, http.MethodDelete, path, nil)
	return err
}

// Logs returns the tail of a container's logs.
func (s *Service) Logs(ctx context.Context, id string, tail int) (string, error) {
	if !validRef(id) {
		return "", fmt.Errorf("非法容器 ID")
	}
	if s.Dev() {
		return fmt.Sprintf("[privacyos-demo] %s 容器日志\n2026-05-12T09:12:01Z  INFO  service started\n"+
			"2026-05-12T09:12:02Z  INFO  listening on 0.0.0.0\n2026-05-12T09:20:11Z  INFO  healthcheck ok\n", id), nil
	}
	if tail <= 0 || tail > 5000 {
		tail = 300
	}
	path := fmt.Sprintf("/containers/%s/logs?stdout=1&stderr=1&timestamps=1&tail=%d", id, tail)
	body, err := s.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	return demux(body), nil
}

// Stats returns a container resource sample.
func (s *Service) Stats(ctx context.Context, id string) (*Stats, error) {
	if !validRef(id) {
		return nil, fmt.Errorf("非法容器 ID")
	}
	if s.Dev() {
		return &Stats{CPU: 2.4, MemUsage: 184 << 20, MemLimit: 2 << 30, NetRx: 1 << 20, NetTx: 256 << 10, BlockRead: 4 << 20, BlockWrite: 1 << 20}, nil
	}
	body, err := s.do(ctx, http.MethodGet, "/containers/"+id+"/stats?stream=false", nil)
	if err != nil {
		return nil, err
	}
	var doc struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage  uint64   `json:"total_usage"`
				PercpuUsage []uint64 `json:"percpu_usage"`
			} `json:"cpu_usage"`
			SystemUsage uint64 `json:"system_cpu_usage"`
			OnlineCPUs  uint32 `json:"online_cpus"`
		} `json:"cpu_stats"`
		PreCPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemUsage uint64 `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		MemoryStats struct {
			Usage uint64 `json:"usage"`
			Limit uint64 `json:"limit"`
			Stats struct {
				Cache uint64 `json:"cache"`
			} `json:"stats"`
		} `json:"memory_stats"`
		Networks map[string]struct {
			RxBytes uint64 `json:"rx_bytes"`
			TxBytes uint64 `json:"tx_bytes"`
		} `json:"networks"`
		BlkioStats struct {
			IoServiceBytesRecursive []struct {
				Op    string `json:"op"`
				Value uint64 `json:"value"`
			} `json:"io_service_bytes_recursive"`
		} `json:"blkio_stats"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	st := &Stats{}
	cpuDelta := float64(doc.CPUStats.CPUUsage.TotalUsage) - float64(doc.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(doc.CPUStats.SystemUsage) - float64(doc.PreCPUStats.SystemUsage)
	cpus := float64(doc.CPUStats.OnlineCPUs)
	if cpus == 0 {
		cpus = float64(len(doc.CPUStats.CPUUsage.PercpuUsage))
	}
	if sysDelta > 0 && cpuDelta > 0 {
		st.CPU = float64(int(cpuDelta/sysDelta*cpus*1000+0.5)) / 10
	}
	st.MemUsage = int64(doc.MemoryStats.Usage)
	st.MemLimit = int64(doc.MemoryStats.Limit)
	for _, n := range doc.Networks {
		st.NetRx += int64(n.RxBytes)
		st.NetTx += int64(n.TxBytes)
	}
	for _, b := range doc.BlkioStats.IoServiceBytesRecursive {
		switch strings.ToLower(b.Op) {
		case "read":
			st.BlockRead += int64(b.Value)
		case "write":
			st.BlockWrite += int64(b.Value)
		}
	}
	return st, nil
}

// ListImages returns local images.
func (s *Service) ListImages(ctx context.Context) ([]Image, error) {
	if s.Dev() {
		return []Image{
			{ID: "sha256:8f1a", Tags: []string{"jellyfin/jellyfin:latest"}, Size: 1_240_000_000, Created: 1717000000},
			{ID: "sha256:2b7c", Tags: []string{"nextcloud:latest"}, Size: 980_000_000, Created: 1716500000},
			{ID: "sha256:c4d9", Tags: []string{"lscr.io/linuxserver/qbittorrent:latest"}, Size: 320_000_000, Created: 1716000000},
			{ID: "sha256:9e11", Tags: []string{"adguard/adguardhome:latest"}, Size: 72_000_000, Created: 1715400000},
		}, nil
	}
	var raw []struct {
		ID       string   `json:"Id"`
		RepoTags []string `json:"RepoTags"`
		Size     int64    `json:"Size"`
		Created  int64    `json:"Created"`
	}
	if err := s.getJSON(ctx, "/images/json", &raw); err != nil {
		return nil, err
	}
	out := make([]Image, 0, len(raw))
	for _, im := range raw {
		out = append(out, Image{ID: im.ID, Tags: im.RepoTags, Size: im.Size, Created: im.Created})
	}
	return out, nil
}

// PullImage pulls an image and reports progress.
func (s *Service) PullImage(ctx context.Context, ref string, report func(int, string)) error {
	if !validImageRef(ref) {
		return fmt.Errorf("非法镜像名")
	}
	if s.Dev() {
		report(30, "Pulling from library")
		time.Sleep(300 * time.Millisecond)
		report(70, "Downloading layer")
		time.Sleep(300 * time.Millisecond)
		return nil
	}
	name, tag := ref, "latest"
	if idx := strings.LastIndex(ref, ":"); idx > strings.LastIndex(ref, "/") {
		name, tag = ref[:idx], ref[idx+1:]
	}
	body, err := s.do(ctx, http.MethodPost,
		"/images/create?fromImage="+urlQuery(name)+"&tag="+urlQuery(tag), nil)
	if err != nil {
		return err
	}
	sc := bufio.NewScanner(bytes.NewReader(body))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	n := 0
	for sc.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		n++
		var ev struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		}
		if json.Unmarshal(sc.Bytes(), &ev) == nil {
			if ev.Error != "" {
				return fmt.Errorf("%s", ev.Error)
			}
			if ev.Status != "" && n%5 == 0 {
				report(min(95, 10+n/10), ev.Status)
			}
		}
	}
	return sc.Err()
}

// RemoveImage deletes an image.
func (s *Service) RemoveImage(ctx context.Context, id string) error {
	if !validRef(id) {
		return fmt.Errorf("非法镜像 ID")
	}
	if s.Dev() {
		return nil
	}
	_, err := s.do(ctx, http.MethodDelete, "/images/"+id+"?force=1", nil)
	return err
}

func (s *Service) getJSON(ctx context.Context, path string, dst any) error {
	body, err := s.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dst)
}

func (s *Service) do(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, "http://docker"+path, body)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接 Docker 失败: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return data, fmt.Errorf("Docker API %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

// demux strips Docker's multiplexed stream headers.
func demux(data []byte) string {
	var out bytes.Buffer
	for len(data) >= 8 {
		streamType := data[0]
		if streamType > 2 {
			return string(data)
		}
		size := binary.BigEndian.Uint32(data[4:8])
		if int(size) > len(data)-8 {
			return string(data)
		}
		out.Write(data[8 : 8+size])
		data = data[8+size:]
	}
	if out.Len() == 0 {
		return string(data)
	}
	return out.String()
}

func validRef(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F' || r >= '0' && r <= '9' ||
			r == '_' || r == '-' || r == '.') {
			return false
		}
	}
	return true
}

func validImageRef(s string) bool {
	if s == "" || len(s) > 200 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			strings.ContainsRune("_-./:@", r)) {
			return false
		}
	}
	return true
}

func urlQuery(s string) string {
	return strings.NewReplacer("/", "%2F", ":", "%3A", "@", "%40").Replace(s)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ---------- compose ----------

// ComposeProject is a compose stack on disk.
type ComposeProject struct {
	Name     string   `json:"name"`
	File     string   `json:"file"`
	Services []string `json:"services"`
	Updated  string   `json:"updated"`
}

// ListCompose lists compose projects.
func (s *Service) ListCompose() ([]ComposeProject, error) {
	if err := os.MkdirAll(s.composeDir, 0o750); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.composeDir)
	if err != nil {
		return nil, err
	}
	out := []ComposeProject{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		file := filepath.Join(s.composeDir, e.Name(), "docker-compose.yml")
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		body, _ := os.ReadFile(file)
		out = append(out, ComposeProject{
			Name: e.Name(), File: file,
			Services: parseComposeServices(string(body)),
			Updated:  info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

// CreateCompose validates and stores a compose file.
func (s *Service) CreateCompose(name, content string) (*ComposeProject, error) {
	if !validComposeName(name) {
		return nil, fmt.Errorf("非法项目名（仅允许字母、数字、- 和 _）")
	}
	if len(strings.TrimSpace(content)) == 0 {
		return nil, fmt.Errorf("compose 内容为空")
	}
	services := parseComposeServices(content)
	if len(services) == 0 {
		return nil, fmt.Errorf("未找到 services 定义")
	}
	dir := filepath.Join(s.composeDir, name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	file := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(file, []byte(content), 0o640); err != nil {
		return nil, err
	}
	return &ComposeProject{Name: name, File: file, Services: services, Updated: time.Now().UTC().Format(time.RFC3339)}, nil
}

// DeleteCompose removes a compose project after bringing it down.
func (s *Service) DeleteCompose(ctx context.Context, name string) error {
	if !validComposeName(name) {
		return fmt.Errorf("非法项目名")
	}
	dir := filepath.Join(s.composeDir, name)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("项目不存在")
	}
	if !s.Dev() {
		_, _ = s.runCompose(ctx, name, "down", "--remove-orphans")
	}
	return os.RemoveAll(dir)
}

// ComposeUp deploys a project.
func (s *Service) ComposeUp(ctx context.Context, name string) (string, error) {
	if s.Dev() {
		return "开发模式：模拟部署成功", nil
	}
	return s.runCompose(ctx, name, "up", "-d", "--remove-orphans")
}

// ComposeDown stops a project.
func (s *Service) ComposeDown(ctx context.Context, name string) (string, error) {
	if s.Dev() {
		return "开发模式：模拟停止成功", nil
	}
	return s.runCompose(ctx, name, "down")
}

// ComposeLogs returns project logs.
func (s *Service) ComposeLogs(ctx context.Context, name string, tail int) (string, error) {
	if !validComposeName(name) {
		return "", fmt.Errorf("非法项目名")
	}
	if s.Dev() {
		return "[privacyos-demo] compose 项目 " + name + " 运行正常", nil
	}
	if tail <= 0 || tail > 2000 {
		tail = 200
	}
	return s.runCompose(ctx, name, "logs", "--tail", strconv.Itoa(tail), "--no-color")
}

func (s *Service) composeFile(name string) string {
	return filepath.Join(s.composeDir, name, "docker-compose.yml")
}

func (s *Service) runCompose(ctx context.Context, name string, args ...string) (string, error) {
	file := s.composeFile(name)
	if _, err := os.Stat(file); err != nil {
		return "", fmt.Errorf("compose 文件不存在")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	bin, baseArgs := composeCommand()
	if bin == "" {
		return "", fmt.Errorf("未安装 docker compose")
	}
	full := append(baseArgs, "-f", file, "-p", "privacyos-"+name)
	full = append(full, args...)
	cmd := exec.CommandContext(ctx, bin, full...)
	cmd.Dir = filepath.Dir(file)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func composeCommand() (string, []string) {
	if _, err := exec.LookPath("docker"); err == nil {
		return "docker", []string{"compose"}
	}
	if _, err := exec.LookPath("docker-compose"); err == nil {
		return "docker-compose", nil
	}
	return "", nil
}

var reService = regexp.MustCompile(`(?m)^\s{2}([a-zA-Z0-9][a-zA-Z0-9_.-]*):\s*$`)

func parseComposeServices(content string) []string {
	inServices := false
	var out []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "services:" {
			inServices = true
			continue
		}
		if inServices {
			if len(line) > 0 && line[0] != ' ' && trimmed != "" {
				break
			}
			if m := reService.FindStringSubmatch(line); m != nil {
				out = append(out, m[1])
			}
		}
	}
	return out
}

func validComposeName(s string) bool {
	if s == "" || len(s) > 60 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func demoContainers() []Container {
	return []Container{
		{ID: "a1b2c3d4e5f6", Name: "privacyos-jellyfin", Image: "jellyfin/jellyfin:latest", State: "running",
			Status: "Up 3 days", Created: 1716500000, CPU: 1.4, Memory: 412 << 20,
			Ports: []Port{{Private: 8096, Public: 8096, Type: "tcp"}}},
		{ID: "b2c3d4e5f6a1", Name: "privacyos-qbittorrent", Image: "lscr.io/linuxserver/qbittorrent:latest",
			State: "running", Status: "Up 3 days", Created: 1716400000, CPU: 0.8, Memory: 186 << 20,
			Ports: []Port{{Private: 8081, Public: 8081, Type: "tcp"}, {Private: 6881, Public: 6881, Type: "tcp"}}},
		{ID: "c3d4e5f6a1b2", Name: "privacyos-adguardhome", Image: "adguard/adguardhome:latest", State: "exited",
			Status: "Exited (0) 2 hours ago", Created: 1716300000,
			Ports: []Port{{Private: 3000, Public: 3000, Type: "tcp"}, {Private: 53, Public: 53, Type: "udp"}}},
		{ID: "d4e5f6a1b2c3", Name: "privacyos-uptime-kuma", Image: "louislam/uptime-kuma:1", State: "running",
			Status: "Up 12 hours", Created: 1716200000, CPU: 0.3, Memory: 96 << 20,
			Ports: []Port{{Private: 3001, Public: 3001, Type: "tcp"}}},
	}
}
