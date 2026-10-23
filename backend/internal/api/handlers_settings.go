package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"privacyos/backend/internal/core"
	"privacyos/backend/internal/files"
	"privacyos/backend/internal/store"
)

// settingsKeys is the writable settings whitelist.
var settingsKeys = map[string]bool{
	"security": true, "destroy": true, "deletion": true, "tmdb": true,
	"notifications": true, "network": true, "ddns": true, "system": true,
	"normal": true, "https": true, "backup": true,
}

// getSettings returns all settings with secrets masked.
func (s *Server) getSettings(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	st := s.maskedSettings()
	ok(c, gin.H{
		"settings": st, "edition": s.Cfg.Edition, "edition_name": s.Cfg.EditionName(),
		"pro": s.Cfg.Pro(), "version": s.Cfg.Version, "dev": s.Cfg.Dev,
		"paths": gin.H{
			"data": s.Cfg.DataDir, "config": s.Cfg.ConfigDir, "logs": s.Cfg.LogDir,
			"cache": s.Cfg.CacheDir, "web": s.Cfg.WebDir, "database": s.Cfg.DBPath,
		},
	})
}

// maskedSettings returns settings with credentials replaced by a sentinel.
func (s *Server) maskedSettings() core.Settings {
	st := s.App.LoadSettings()
	if st.Notifications.SMTPPassword != "" {
		st.Notifications.SMTPPassword = keepSecret
	}
	if st.Notifications.TelegramToken != "" {
		st.Notifications.TelegramToken = keepSecret
	}
	if st.Notifications.ServerChanKey != "" {
		st.Notifications.ServerChanKey = keepSecret
	}
	if st.Notifications.BarkURL != "" {
		st.Notifications.BarkURL = keepSecret
	}
	if st.TMDB.APIKey != "" {
		st.TMDB.APIKey = keepSecret
	}
	if st.DDNS.Token != "" {
		st.DDNS.Token = keepSecret
	}
	if st.DDNS.Password != "" {
		st.DDNS.Password = keepSecret
	}
	return st
}

// putSetting merges and validates one settings section.
func (s *Server) putSetting(c *gin.Context) {
	s.saveSetting(c, c.Param("key"))
}

// saveSetting merges and validates one settings section.
func (s *Server) saveSetting(c *gin.Context, key string) {
	if !s.requireAdmin(c) {
		return
	}
	if !settingsKeys[key] {
		apiError(c, http.StatusBadRequest, "unknown_setting", "未知的设置项")
		return
	}
	if key == "destroy" && !s.Pro() {
		apiError(c, http.StatusBadRequest, "edition_mismatch", "销毁策略仅专业版可用")
		return
	}
	var patch map[string]any
	if err := c.ShouldBindJSON(&patch); err != nil {
		apiError(c, http.StatusBadRequest, "bad_request", "请求参数不正确")
		return
	}
	st := s.App.LoadSettings()
	var target any
	switch key {
	case "security":
		target = &st.Security
	case "destroy":
		target = &st.Destroy
	case "deletion":
		target = &st.Deletion
	case "tmdb":
		target = &st.TMDB
	case "notifications":
		target = &st.Notifications
	case "network":
		target = &st.Network
	case "ddns":
		target = &st.DDNS
	case "system":
		target = &st.System
	case "normal":
		target = &st.Normal
	case "https":
		target = &st.HTTPS
	case "backup":
		target = &st.Backup
	}
	if err := mergeJSON(target, patch); err != nil {
		apiError(c, http.StatusBadRequest, "bad_request", "参数格式不正确")
		return
	}
	if msg := validateSettings(key, st); msg != "" {
		apiError(c, http.StatusBadRequest, "invalid_setting", msg)
		return
	}
	if key == "system" && strings.TrimSpace(st.System.Name) == "" {
		st.System.Name = "PrivacyOS"
	}
	if err := s.Store.SetSetting(key, target); err != nil {
		apiError(c, http.StatusInternalServerError, "save_failed", "保存设置失败")
		return
	}
	s.audit(c, "system", "settings_update", key, "更新系统设置", "ok")
	ok(c, gin.H{"message": "设置已保存", "settings": s.maskedSettings()})
}

// validateSettings checks numeric ranges and enum values.
func validateSettings(key string, st core.Settings) string {
	switch key {
	case "security":
		sec := st.Security
		if sec.MaxAttempts < 3 || sec.MaxAttempts > 50 {
			return "锁定阈值需在 3-50 之间"
		}
		if sec.WindowSeconds < 60 || sec.WindowSeconds > 86400 {
			return "统计窗口需在 60-86400 秒之间"
		}
		if sec.LockSeconds < 60 || sec.LockSeconds > 86400 {
			return "锁定时长需在 60-86400 秒之间"
		}
		if sec.DelayFrom < 1 || sec.DelayFrom > sec.MaxAttempts {
			return "递增延迟起始次数不合法"
		}
		if sec.BaseDelaySeconds < 1 || sec.MaxDelaySeconds < sec.BaseDelaySeconds {
			return "延迟参数不合法"
		}
	case "destroy":
		d := st.Destroy
		if d.Threshold < 3 || d.Threshold > 50 {
			return "销毁阈值需在 3-50 之间"
		}
		if d.WindowSeconds < 60 || d.WindowSeconds > 86400 {
			return "时间窗口需在 60-86400 秒之间"
		}
		switch d.Method {
		case "key_destroy", "overwrite", "delete":
		default:
			return "销毁方式不合法"
		}
		switch d.Fallback {
		case "none", "overwrite", "delete":
		default:
			return "回退销毁方式不合法"
		}
		if d.VaultDevice != "" && !strings.HasPrefix(d.VaultDevice, "/dev/") {
			return "加密卷设备路径必须以 /dev/ 开头"
		}
	case "deletion":
		if st.Deletion.CoolingDays < 0 || st.Deletion.CoolingDays > 365 {
			return "冷静期需在 0-365 天之间"
		}
		if st.Deletion.TrashRetentionDays < 1 || st.Deletion.TrashRetentionDays > 365 {
			return "回收站保留天数需在 1-365 之间"
		}
	case "notifications":
		n := st.Notifications
		if n.SMTPPort < 1 || n.SMTPPort > 65535 {
			return "SMTP 端口不合法"
		}
		if n.EmailEnabled && (n.SMTPHost == "" || n.SMTPTo == "") {
			return "启用邮件通知需要填写 SMTP 主机和收件人"
		}
	case "https":
		switch st.HTTPS.Mode {
		case "selfsigned", "custom", "letsencrypt":
		default:
			return "HTTPS 模式不合法"
		}
		if st.HTTPS.Enabled && st.HTTPS.Mode == "custom" && (st.HTTPS.Cert == "" || st.HTTPS.Key == "") {
			return "自定义证书需要填写证书和私钥路径"
		}
	case "backup":
		if st.Backup.Keep < 1 || st.Backup.Keep > 100 {
			return "备份保留份数需在 1-100 之间"
		}
	}
	return ""
}

// systemInfo returns host information.
func (s *Server) systemInfo(c *gin.Context) {
	info := s.System.Info(c.Request.Context())
	st := s.App.LoadSettings()
	ok(c, gin.H{"info": info, "system": st.System, "network": st.Network,
		"https": gin.H{"enabled": st.HTTPS.Enabled, "mode": st.HTTPS.Mode, "cert": s.Cfg.TLSCert != ""},
		"paths": gin.H{"data": s.Cfg.DataDir, "config": s.Cfg.ConfigDir}})
}

// systemUpdates lists available package upgrades.
func (s *Server) systemUpdates(c *gin.Context) {
	updates, err := s.System.Updates(c.Request.Context())
	if err != nil {
		apiError(c, http.StatusBadGateway, "updates_error", err.Error())
		return
	}
	ok(c, updates)
}

// systemApplyUpdates starts an upgrade job.
func (s *Server) systemApplyUpdates(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	job := s.App.Jobs.Start("系统更新", func(ctx context.Context, report func(int, string)) error {
		if err := s.System.ApplyUpdates(ctx, report); err != nil {
			return err
		}
		return nil
	})
	s.audit(c, "system", "updates_apply", "-", "开始系统更新", "ok")
	created(c, job)
}

// systemBackup streams a configuration backup.
func (s *Server) systemBackup(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	path, err := s.System.CreateBackup()
	if err != nil {
		apiError(c, http.StatusInternalServerError, "backup_failed", err.Error())
		return
	}
	defer os.Remove(path)
	s.audit(c, "system", "backup", filepath.Base(path), "导出配置备份", "ok")
	c.FileAttachment(path, filepath.Base(path))
}

// systemRestore restores configuration from an uploaded archive.
func (s *Server) systemRestore(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<30)
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		apiError(c, http.StatusBadRequest, "bad_request", "备份文件过大或格式不正确")
		return
	}
	if c.PostForm("confirm") != "RESTORE" {
		apiError(c, http.StatusBadRequest, "confirm_required", "恢复配置需要 confirm=RESTORE")
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		apiError(c, http.StatusBadRequest, "no_file", "请上传备份文件")
		return
	}
	src, err := fh.Open()
	if err != nil {
		apiError(c, http.StatusBadRequest, "no_file", "无法读取备份文件")
		return
	}
	defer src.Close()
	tmp := filepath.Join(s.Cfg.CacheDir, "tmp", "restore-"+store.RandomToken(6)+".tar.gz")
	if err := os.MkdirAll(filepath.Dir(tmp), 0o750); err != nil {
		apiError(c, http.StatusInternalServerError, "restore_failed", err.Error())
		return
	}
	out, err := os.Create(tmp)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "restore_failed", err.Error())
		return
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		os.Remove(tmp)
		apiError(c, http.StatusBadRequest, "restore_failed", "写入临时文件失败")
		return
	}
	out.Close()
	defer os.Remove(tmp)
	if err := s.System.RestoreBackup(tmp); err != nil {
		apiError(c, http.StatusBadRequest, "restore_failed", err.Error())
		return
	}
	s.audit(c, "system", "restore", filepath.Base(tmp), "从备份恢复配置", "ok")
	ok(c, gin.H{"message": "配置已恢复，请重启 privacyos 服务以生效", "restart_required": true})
}

// sslInfo returns TLS configuration state.
func (s *Server) sslInfo(c *gin.Context) {
	st := s.App.LoadSettings()
	certPath := s.Cfg.TLSCert
	if certPath == "" {
		certPath = filepath.Join(s.Cfg.ConfigDir, "tls", "server.crt")
	}
	_, err := os.Stat(certPath)
	ok(c, gin.H{
		"enabled": st.HTTPS.Enabled, "mode": st.HTTPS.Mode, "domain": st.HTTPS.Domain,
		"cert_path": certPath, "cert_exists": err == nil,
		"serving_tls": s.Cfg.TLSCert != "",
	})
}

// sslSelfSigned generates a self-signed certificate.
func (s *Server) sslSelfSigned(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	var req struct {
		Domain string `json:"domain"`
	}
	_ = c.ShouldBindJSON(&req)
	cert, key, err := s.System.GenerateSelfSigned(c.Request.Context(), req.Domain)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "cert_failed", err.Error())
		return
	}
	st := s.App.LoadSettings()
	st.HTTPS.Enabled = true
	st.HTTPS.Mode = "selfsigned"
	if req.Domain != "" {
		st.HTTPS.Domain = req.Domain
	}
	st.HTTPS.Cert, st.HTTPS.Key = cert, key
	_ = s.Store.SetSetting("https", st.HTTPS)
	s.audit(c, "system", "ssl_selfsigned", req.Domain, "生成自签名证书", "ok")
	ok(c, gin.H{"message": "证书已生成，重启服务后启用 HTTPS", "cert": cert, "key": key})
}

// firewallGet returns firewall status and the default ruleset.
func (s *Server) firewallGet(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	ok(c, gin.H{
		"status": s.System.FirewallStatus(c.Request.Context()),
		"default_ruleset": s.System.RenderFirewall(nil),
		"active": s.System.FirewallStatus(c.Request.Context()) != "",
	})
}

// firewallPut applies an nftables ruleset.
func (s *Server) firewallPut(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	var req struct {
		Ruleset    string `json:"ruleset"`
		ExtraPorts []int  `json:"extra_ports"`
		Confirm    string `json:"confirm"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if req.Confirm != "APPLY" {
		apiError(c, http.StatusBadRequest, "confirm_required", "应用防火墙需要 confirm=APPLY")
		return
	}
	ruleset := req.Ruleset
	if strings.TrimSpace(ruleset) == "" {
		ruleset = s.System.RenderFirewall(req.ExtraPorts)
	}
	if err := s.System.ApplyFirewall(c.Request.Context(), ruleset); err != nil {
		apiError(c, http.StatusBadRequest, "firewall_error", err.Error())
		return
	}
	s.audit(c, "system", "firewall_apply", "-", "应用防火墙规则", "ok")
	ok(c, gin.H{"message": "防火墙规则已应用", "warning": "120 秒内未确认将自动回滚"})
}

// firewallConfirm cancels the automatic firewall rollback.
func (s *Server) firewallConfirm(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	if err := s.System.ConfirmFirewall(c.Request.Context()); err != nil {
		apiError(c, http.StatusBadRequest, "firewall_error", err.Error())
		return
	}
	s.audit(c, "system", "firewall_confirm", "-", "确认保留防火墙规则", "ok")
	ok(c, gin.H{"message": "已确认，规则将保留"})
}

// encryptionInfo returns encryption status.
func (s *Server) encryptionInfo(c *gin.Context) {
	info := s.System.Encryption(c.Request.Context())
	ok(c, gin.H{"encryption": info, "pro": s.Pro()})
}

// vaultList lists per-user encrypted vaults.
func (s *Server) vaultList(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	users, err := s.Store.ListUsers("", false)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取用户失败")
		return
	}
	out := make([]core.VaultInfo, 0, len(users))
	for _, u := range users {
		out = append(out, s.App.VaultInfo(u.Username))
	}
	ok(c, gin.H{"vaults": out, "available": s.App.VaultAvailable(), "dev": s.Cfg.Dev,
		"encryption": s.System.Encryption(c.Request.Context())})
}

// vaultCreate creates an encrypted home for a user.
func (s *Server) vaultCreate(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	if !s.Pro() {
		apiError(c, http.StatusBadRequest, "edition_mismatch", "加密卷仅专业版可用")
		return
	}
	username := c.Param("username")
	u, err := s.Store.GetUserByUsername(username)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "用户不存在")
		return
	}
	if s.App.VaultExists(username) {
		apiError(c, http.StatusConflict, "vault_exists", "该用户已存在加密卷")
		return
	}
	var req struct {
		SizeGB int `json:"size_gb"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.SizeGB < 1 {
		req.SizeGB = 50
	}
	if req.SizeGB > 65536 {
		apiError(c, http.StatusBadRequest, "invalid_size", "容量超出范围")
		return
	}
	home := s.Files.Home(u)
	prevault := home + ".prevault"
	if _, err := os.Stat(home); err == nil {
		if err := os.Rename(home, prevault); err != nil {
			apiError(c, http.StatusInternalServerError, "vault_failed", "迁移用户目录失败: "+err.Error())
			return
		}
	}
	if err := s.App.CreateVault(c.Request.Context(), u, int64(req.SizeGB)<<30); err != nil {
		_ = os.Rename(prevault, home)
		apiError(c, http.StatusInternalServerError, "vault_failed", err.Error())
		return
	}
	if _, err := os.Stat(prevault); err == nil {
		if err := files.CopyTree(prevault, home); err != nil {
			_ = s.App.LockVault(c.Request.Context(), username)
			_ = os.RemoveAll(home)
			_ = os.Rename(prevault, home)
			apiError(c, http.StatusInternalServerError, "vault_failed", "迁移已有文件失败: "+err.Error())
			return
		}
		_ = os.RemoveAll(prevault)
	}
	s.audit(c, "system", "vault_create", username, fmt.Sprintf("创建 %d GB 加密卷", req.SizeGB), "ok")
	created(c, s.App.VaultInfo(username))
}

// vaultUnlock opens a user vault.
func (s *Server) vaultUnlock(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	username := c.Param("username")
	if err := s.App.UnlockVault(c.Request.Context(), username); err != nil {
		apiError(c, http.StatusBadRequest, "vault_failed", err.Error())
		return
	}
	s.audit(c, "system", "vault_unlock", username, "解锁加密卷", "ok")
	ok(c, s.App.VaultInfo(username))
}

// vaultLock closes a user vault.
func (s *Server) vaultLock(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	username := c.Param("username")
	if err := s.App.LockVault(c.Request.Context(), username); err != nil {
		apiError(c, http.StatusBadRequest, "vault_failed", err.Error())
		return
	}
	s.audit(c, "system", "vault_lock", username, "锁定加密卷", "ok")
	ok(c, s.App.VaultInfo(username))
}

// manualDestroy triggers a manual destruction (Pro only).
func (s *Server) manualDestroy(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	if !s.Pro() {
		apiError(c, http.StatusBadRequest, "edition_mismatch", "销毁功能仅专业版可用")
		return
	}
	var req struct {
		Username string `json:"username" binding:"required"`
		Confirm  string `json:"confirm" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if req.Confirm != req.Username {
		apiError(c, http.StatusBadRequest, "confirm_required", "请输入用户名以确认销毁")
		return
	}
	if _, err := s.Store.GetUserByUsername(req.Username); err != nil {
		apiError(c, http.StatusNotFound, "not_found", "用户不存在")
		return
	}
	job := s.App.TriggerDestroy(req.Username, "manual")
	s.audit(c, "destroy", "manual_destroy", req.Username, "手动触发数据销毁", "ok")
	created(c, job)
}

// destroyPolicyGet returns the destruction policy (Pro only).
func (s *Server) destroyPolicyGet(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	st := s.App.LoadSettings()
	ok(c, gin.H{"policy": st.Destroy, "pro": s.Pro(), "vault_available": s.App.VaultAvailable()})
}

// destroyPolicyPut updates the destruction policy.
func (s *Server) destroyPolicyPut(c *gin.Context) {
	if !s.Pro() {
		apiError(c, http.StatusBadRequest, "edition_mismatch", "销毁策略仅专业版可用")
		return
	}
	s.saveSetting(c, "destroy")
}

// notificationsList returns in-app notifications.
func (s *Server) notificationsList(c *gin.Context) {
	items, err := s.Store.ListNotifications(intParam(c, "limit", 50), c.Query("unread") == "1")
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取通知失败")
		return
	}
	unread, _ := s.Store.UnreadCount()
	ok(c, gin.H{"items": items, "unread": unread})
}

// notificationsRead marks notifications as read.
func (s *Server) notificationsRead(c *gin.Context) {
	var req struct {
		ID int64 `json:"id"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := s.Store.MarkNotificationsRead(req.ID); err != nil {
		apiError(c, http.StatusInternalServerError, "update_failed", "更新失败")
		return
	}
	ok(c, gin.H{"message": "已标记为已读"})
}

// notificationDelete removes a notification.
func (s *Server) notificationDelete(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := s.Store.DeleteNotification(id); err != nil {
		apiError(c, http.StatusInternalServerError, "delete_failed", "删除失败")
		return
	}
	ok(c, gin.H{"message": "已删除"})
}

// notificationTest sends a test message through all channels.
func (s *Server) notificationTest(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	results := s.App.Notify.Send(c.Request.Context(), "PrivacyOS 测试通知",
		"这是一条来自 PrivacyOS 的测试通知，如果你收到它，说明通知渠道配置正确。")
	ok(c, results)
}
