package api

import (
	"net"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"privacyos/backend/internal/core"
	"privacyos/backend/internal/store"
)

// networkInterfaces lists NICs.
func (s *Server) networkInterfaces(c *gin.Context) {
	ifaces, err := s.System.Interfaces(c.Request.Context())
	if err != nil {
		apiError(c, http.StatusInternalServerError, "network_error", err.Error())
		return
	}
	settings := s.App.LoadSettings()
	ok(c, gin.H{"interfaces": ifaces, "config": settings.Network})
}

// networkApplyInterface applies a static/DHCP configuration.
func (s *Server) networkApplyInterface(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	var req struct {
		Mode    string `json:"mode" binding:"required"`
		Address string `json:"address"`
		Gateway string `json:"gateway"`
		DNS     string `json:"dns"`
		Confirm string `json:"confirm"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if req.Confirm != "APPLY" {
		apiError(c, http.StatusBadRequest, "confirm_required", "修改网络可能导致连接中断，请填写 confirm=APPLY")
		return
	}
	name := c.Param("name")
	if err := s.System.ApplyInterface(c.Request.Context(), name, req.Mode, req.Address, req.Gateway, req.DNS); err != nil {
		apiError(c, http.StatusBadRequest, "network_error", err.Error())
		return
	}
	settings := s.App.LoadSettings()
	settings.Network.Interfaces[name] = core.InterfaceConfig{
		Mode: req.Mode, Address: req.Address, Gateway: req.Gateway, DNS: req.DNS,
	}
	_ = s.Store.SetSetting("network", settings.Network)
	s.audit(c, "network", "interface_apply", name, "应用网络配置 "+req.Mode, "ok")
	ok(c, gin.H{"message": "网络配置已应用"})
}

// networkPorts lists listening sockets.
func (s *Server) networkPorts(c *gin.Context) {
	ports, err := s.System.Ports(c.Request.Context())
	if err != nil {
		apiError(c, http.StatusInternalServerError, "network_error", err.Error())
		return
	}
	ok(c, ports)
}

// networkSharesList lists SMB/NFS exports.
func (s *Server) networkSharesList(c *gin.Context) {
	shares, err := s.Store.ListNetworkShares()
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取共享失败")
		return
	}
	settings := s.App.LoadSettings()
	ok(c, gin.H{"shares": shares, "samba_path": s.Shares.SambaPath(),
		"exports_path": s.Shares.ExportsPath(), "hostname": settings.Network.Hostname})
}

type networkShareRequest struct {
	Name       string   `json:"name" binding:"required"`
	Path       string   `json:"path" binding:"required"`
	Comment    string   `json:"comment"`
	Protocols  []string `json:"protocols"`
	ReadOnly   bool     `json:"read_only"`
	ValidUsers string   `json:"valid_users"`
	GuestOK    bool     `json:"guest_ok"`
}

func (s *Server) validateSharePath(path string) error {
	if !filepath.IsAbs(path) || strings.Contains(path, "..") {
		return errInvalidSharePath
	}
	root := filepath.Clean(s.Cfg.DataDir)
	clean := filepath.Clean(path)
	if clean != root && !strings.HasPrefix(clean, root+string(filepath.Separator)) {
		return errInvalidSharePath
	}
	return nil
}

// networkShareCreate adds an export.
func (s *Server) networkShareCreate(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	var req networkShareRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := s.validateSharePath(req.Path); err != nil {
		apiError(c, http.StatusBadRequest, "invalid_path", "共享路径必须位于数据目录 /data 下")
		return
	}
	if len(req.Protocols) == 0 {
		req.Protocols = []string{"smb"}
	}
	_ = s.Shares.EnsureDirectory(req.Path)
	share := &store.NetworkShare{
		Name: strings.TrimSpace(req.Name), Path: req.Path, Comment: req.Comment,
		Protocols: req.Protocols, ReadOnly: req.ReadOnly, ValidUsers: req.ValidUsers, GuestOK: req.GuestOK,
	}
	if err := s.Store.CreateNetworkShare(share); err != nil {
		apiError(c, http.StatusBadRequest, "create_failed", "创建共享失败（名称可能重复）")
		return
	}
	res, applyErr := s.Shares.Apply(c.Request.Context(), s.Pro())
	s.audit(c, "network", "share_create", share.Name, "新增共享目录", "ok")
	created(c, gin.H{"share": share, "apply": res, "apply_error": errText(applyErr)})
}

// networkShareUpdate edits an export.
func (s *Server) networkShareUpdate(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	share, err := s.Store.GetNetworkShare(c.Param("id"))
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "共享不存在")
		return
	}
	var req networkShareRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := s.validateSharePath(req.Path); err != nil {
		apiError(c, http.StatusBadRequest, "invalid_path", "共享路径必须位于数据目录 /data 下")
		return
	}
	share.Name = strings.TrimSpace(req.Name)
	share.Path = req.Path
	share.Comment = req.Comment
	if len(req.Protocols) > 0 {
		share.Protocols = req.Protocols
	}
	share.ReadOnly = req.ReadOnly
	share.ValidUsers = req.ValidUsers
	share.GuestOK = req.GuestOK
	if err := s.Store.UpdateNetworkShare(share); err != nil {
		apiError(c, http.StatusInternalServerError, "update_failed", "更新失败")
		return
	}
	res, applyErr := s.Shares.Apply(c.Request.Context(), s.Pro())
	s.audit(c, "network", "share_update", share.Name, "修改共享目录", "ok")
	ok(c, gin.H{"share": share, "apply": res, "apply_error": errText(applyErr)})
}

// networkShareDelete removes an export.
func (s *Server) networkShareDelete(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	share, err := s.Store.GetNetworkShare(c.Param("id"))
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "共享不存在")
		return
	}
	if err := s.Store.DeleteNetworkShare(share.ID); err != nil {
		apiError(c, http.StatusInternalServerError, "delete_failed", "删除失败")
		return
	}
	_, _ = s.Shares.Apply(c.Request.Context(), s.Pro())
	s.audit(c, "network", "share_delete", share.Name, "删除共享目录", "ok")
	ok(c, gin.H{"message": "共享已删除"})
}

// networkApply reloads Samba/NFS configuration.
func (s *Server) networkApply(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	res, err := s.Shares.Apply(c.Request.Context(), s.Pro())
	if err != nil {
		apiError(c, http.StatusBadRequest, "apply_failed", err.Error())
		return
	}
	s.audit(c, "network", "share_apply", "-", "应用共享配置", "ok")
	ok(c, res)
}

// ddnsGet returns dynamic DNS settings with the token masked.
func (s *Server) ddnsGet(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	settings := s.App.LoadSettings()
	ddns := settings.DDNS
	if ddns.Token != "" {
		ddns.Token = keepSecret
	}
	if ddns.Password != "" {
		ddns.Password = keepSecret
	}
	ok(c, ddns)
}

// ddnsPut saves dynamic DNS settings.
func (s *Server) ddnsPut(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	var req struct {
		Enabled  bool   `json:"enabled"`
		Provider string `json:"provider"`
		Domain   string `json:"domain"`
		Token    string `json:"token"`
		Username string `json:"username"`
		Password string `json:"password"`
		URL      string `json:"url"`
		Interval int    `json:"interval"`
	}
	if !bindJSON(c, &req) {
		return
	}
	settings := s.App.LoadSettings()
	settings.DDNS.Enabled = req.Enabled
	if req.Provider != "" {
		settings.DDNS.Provider = req.Provider
	}
	settings.DDNS.Domain = req.Domain
	settings.DDNS.Username = req.Username
	settings.DDNS.URL = req.URL
	if req.Interval >= 60 {
		settings.DDNS.Interval = req.Interval
	}
	if req.Token != keepSecret {
		settings.DDNS.Token = req.Token
	}
	if req.Password != keepSecret {
		settings.DDNS.Password = req.Password
	}
	if err := s.Store.SetSetting("ddns", settings.DDNS); err != nil {
		apiError(c, http.StatusInternalServerError, "save_failed", "保存失败")
		return
	}
	s.audit(c, "network", "ddns_update", settings.DDNS.Domain, "更新 DDNS 配置", "ok")
	ok(c, gin.H{"message": "DDNS 配置已保存"})
}

// ddnsUpdate triggers an immediate DDNS update.
func (s *Server) ddnsUpdate(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	settings := s.App.LoadSettings()
	if !settings.DDNS.Enabled && c.Query("force") != "1" {
		apiError(c, http.StatusBadRequest, "ddns_disabled", "DDNS 未启用")
		return
	}
	ip, err := s.System.UpdateDDNS(c.Request.Context(), settings.DDNS.Provider, settings.DDNS.Domain,
		settings.DDNS.Token, settings.DDNS.Username, settings.DDNS.Password, settings.DDNS.URL)
	settings.DDNS.LastRun = store.Now()
	if err != nil {
		settings.DDNS.LastErr = err.Error()
	} else {
		settings.DDNS.LastErr = ""
		settings.DDNS.LastIP = ip
	}
	_ = s.Store.SetSetting("ddns", settings.DDNS)
	if err != nil {
		apiError(c, http.StatusBadGateway, "ddns_error", err.Error())
		return
	}
	ok(c, gin.H{"ip": ip, "message": "DDNS 更新成功"})
}

// dnsGet returns DNS settings.
func (s *Server) dnsGet(c *gin.Context) {
	settings := s.App.LoadSettings()
	ok(c, gin.H{"servers": settings.Network.DNS, "hostname": settings.Network.Hostname, "domain": settings.Network.Domain})
}

// dnsPut applies DNS servers.
func (s *Server) dnsPut(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	var req struct {
		Servers []string `json:"servers" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	for _, srv := range req.Servers {
		if net.ParseIP(srv) == nil {
			apiError(c, http.StatusBadRequest, "invalid_dns", "DNS 地址不合法: "+srv)
			return
		}
	}
	settings := s.App.LoadSettings()
	settings.Network.DNS = req.Servers
	if err := s.Store.SetSetting("network", settings.Network); err != nil {
		apiError(c, http.StatusInternalServerError, "save_failed", "保存失败")
		return
	}
	if err := s.System.SetDNS(c.Request.Context(), req.Servers); err != nil {
		apiError(c, http.StatusBadRequest, "dns_error", err.Error())
		return
	}
	s.audit(c, "network", "dns_update", strings.Join(req.Servers, ","), "更新 DNS", "ok")
	ok(c, gin.H{"message": "DNS 已更新"})
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

var errInvalidSharePath = &sharePathError{}

type sharePathError struct{}

func (e *sharePathError) Error() string { return "共享路径必须位于数据目录下" }
