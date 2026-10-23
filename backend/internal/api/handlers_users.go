package api

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"privacyos/backend/internal/auth"
	"privacyos/backend/internal/store"
)

var reUsername = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{2,31}$`)

type createUserRequest struct {
	Username    string             `json:"username" binding:"required"`
	Password    string             `json:"password" binding:"required"`
	DisplayName string             `json:"display_name"`
	Role        string             `json:"role"`
	QuotaBytes  int64              `json:"quota_bytes"`
	Permissions *store.Permissions `json:"permissions"`
}

// listUsers returns all accounts.
func (s *Server) listUsers(c *gin.Context) {
	users, err := s.Store.ListUsers(c.Query("query"), false)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取用户失败")
		return
	}
	viewer := currentUser(c)
	out := make([]store.User, 0, len(users))
	for _, u := range users {
		s.sanitizeUser(viewer, &u)
		out = append(out, u)
	}
	ok(c, out)
}

// sanitizeUser applies edition privacy rules to a user payload. In the Pro
// edition an administrator may not inspect another user's quota.
func (s *Server) sanitizeUser(viewer, u *store.User) {
	if viewer == nil || u == nil {
		return
	}
	if s.Pro() && viewer.ID != u.ID {
		u.QuotaBytes = 0
		u.QuotaHidden = true
	}
}

// createUser provisions a new account.
func (s *Server) createUser(c *gin.Context) {
	var req createUserRequest
	if !bindJSON(c, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !reUsername.MatchString(req.Username) {
		apiError(c, http.StatusBadRequest, "invalid_username", "用户名需为 3-32 位字母、数字、下划线或短横线")
		return
	}
	if req.Role == "" {
		req.Role = "user"
	}
	if req.Role != "admin" && req.Role != "user" {
		apiError(c, http.StatusBadRequest, "invalid_role", "角色只能是 admin 或 user")
		return
	}
	if _, err := s.Store.GetUserByUsername(req.Username); err == nil {
		apiError(c, http.StatusConflict, "user_exists", "用户名已存在")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		apiError(c, http.StatusBadRequest, "weak_password", err.Error())
		return
	}
	perms := store.DefaultPermissions(req.Role)
	if req.Permissions != nil {
		perms = *req.Permissions
	}
	u := &store.User{
		Username: req.Username, DisplayName: strings.TrimSpace(req.DisplayName),
		PasswordHash: hash, Role: req.Role, Status: "active", QuotaBytes: req.QuotaBytes,
		Permissions: perms, Preferences: store.Preferences{Lang: "zh-CN", Theme: "dark"},
	}
	if u.DisplayName == "" {
		u.DisplayName = req.Username
	}
	if err := s.Store.CreateUser(u); err != nil {
		apiError(c, http.StatusInternalServerError, "create_failed", "创建用户失败: "+err.Error())
		return
	}
	// Provision the Unix/Samba account and directories.
	if err := s.System.EnsureLinuxUser(c.Request.Context(), u.Username, req.Password, s.Pro()); err != nil {
		_ = s.Store.DeleteUser(u.ID)
		apiError(c, http.StatusInternalServerError, "system_user_failed", "创建系统账号失败: "+err.Error())
		return
	}
	for _, dir := range []string{
		s.Files.Home(u),
		filepath.Join(s.Cfg.DataDir, "photos", u.Username),
		filepath.Join(s.Cfg.DataDir, "movies", u.Username),
	} {
		perm := os.FileMode(0o755)
		if s.Pro() {
			perm = 0o700
		}
		_ = os.MkdirAll(dir, perm)
	}
	s.audit(c, "user", "create", u.Username, "创建用户", "ok")
	created(c, u)
}

// getUser returns one account.
func (s *Server) getUser(c *gin.Context) {
	id := int64Param(c, "id")
	u, err := s.Store.GetUserByID(id)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "用户不存在")
		return
	}
	s.sanitizeUser(currentUser(c), u)
	ok(c, u)
}

type updateUserRequest struct {
	DisplayName *string             `json:"display_name"`
	Role        *string             `json:"role"`
	Status      *string             `json:"status"`
	QuotaBytes  *int64              `json:"quota_bytes"`
	Permissions *store.Permissions  `json:"permissions"`
}

// updateUser modifies an account.
func (s *Server) updateUser(c *gin.Context) {
	id := int64Param(c, "id")
	u, err := s.Store.GetUserByID(id)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "用户不存在")
		return
	}
	var req updateUserRequest
	if !bindJSON(c, &req) {
		return
	}
	viewer := currentUser(c)
	if req.DisplayName != nil {
		u.DisplayName = strings.TrimSpace(*req.DisplayName)
	}
	if req.Role != nil && *req.Role != u.Role {
		if *req.Role != "admin" && *req.Role != "user" {
			apiError(c, http.StatusBadRequest, "invalid_role", "角色只能是 admin 或 user")
			return
		}
		if u.Role == "admin" && *req.Role != "admin" {
			admins, _ := s.Store.CountUsers("admin")
			if admins <= 1 {
				apiError(c, http.StatusBadRequest, "last_admin", "不能降级唯一的管理员")
				return
			}
			if viewer.ID == u.ID {
				apiError(c, http.StatusBadRequest, "self_demote", "不能降级自己的账号")
				return
			}
		}
		u.Role = *req.Role
	}
	if req.Status != nil {
		if *req.Status != "active" && *req.Status != "disabled" {
			apiError(c, http.StatusBadRequest, "invalid_status", "状态只能是 active 或 disabled")
			return
		}
		if viewer.ID == u.ID && *req.Status != "active" {
			apiError(c, http.StatusBadRequest, "self_disable", "不能停用自己的账号")
			return
		}
		u.Status = *req.Status
	}
	if req.QuotaBytes != nil {
		if *req.QuotaBytes < 0 {
			apiError(c, http.StatusBadRequest, "invalid_quota", "配额不能为负数")
			return
		}
		u.QuotaBytes = *req.QuotaBytes
	}
	if req.Permissions != nil {
		u.Permissions = *req.Permissions
	}
	if err := s.Store.UpdateUser(u); err != nil {
		apiError(c, http.StatusInternalServerError, "update_failed", "更新用户失败")
		return
	}
	if u.Status != "active" {
		_ = s.Store.RevokeUserSessions(u.ID)
	}
	s.audit(c, "user", "update", u.Username, "更新用户资料/权限", "ok")
	ok(c, u)
}

// deleteUser removes an account (immediate in Normal, cooling period in Pro).
func (s *Server) deleteUser(c *gin.Context) {
	id := int64Param(c, "id")
	u, err := s.Store.GetUserByID(id)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "用户不存在")
		return
	}
	viewer := currentUser(c)
	if viewer.ID == u.ID {
		apiError(c, http.StatusBadRequest, "self_delete", "不能删除自己的账号")
		return
	}
	if u.Role == "admin" {
		admins, _ := s.Store.CountUsers("admin")
		if admins <= 1 {
			apiError(c, http.StatusBadRequest, "last_admin", "不能删除唯一的管理员")
			return
		}
	}
	_ = s.Store.RevokeUserSessions(u.ID)
	if s.Pro() {
		settings := s.App.LoadSettings()
		days := settings.Deletion.CoolingDays
		if days <= 0 {
			days = 30
		}
		after := time.Now().AddDate(0, 0, days).UTC().Format(time.RFC3339)
		if err := s.Store.MarkPendingDeletion(u.ID, after); err != nil {
			apiError(c, http.StatusInternalServerError, "delete_failed", "标记删除失败")
			return
		}
		s.audit(c, "user", "delete_pending", u.Username, "进入 "+(time.Duration(days)*24*time.Hour).String()+" 冷静期", "ok")
		ok(c, gin.H{"message": "用户已进入删除冷静期", "delete_after": after, "cooling_days": days})
		return
	}
	// Normal edition: immediate removal.
	_ = s.System.DeleteLinuxUser(c.Request.Context(), u.Username)
	for _, dir := range []string{
		s.Files.Home(u),
		filepath.Join(s.Cfg.DataDir, "photos", u.Username),
		filepath.Join(s.Cfg.DataDir, "movies", u.Username),
		filepath.Join(s.Cfg.DataDir, "trash", u.Username),
	} {
		_ = os.RemoveAll(dir)
	}
	if err := s.Store.DeleteUser(u.ID); err != nil {
		apiError(c, http.StatusInternalServerError, "delete_failed", "删除用户失败")
		return
	}
	s.audit(c, "user", "delete", u.Username, "永久删除用户与数据", "ok")
	ok(c, gin.H{"message": "用户已删除"})
}

// restoreUser cancels a pending deletion.
func (s *Server) restoreUser(c *gin.Context) {
	id := int64Param(c, "id")
	u, err := s.Store.GetUserByID(id)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "用户不存在")
		return
	}
	if u.Status != "pending_deletion" {
		apiError(c, http.StatusBadRequest, "not_pending", "该用户不在冷静期")
		return
	}
	if u.DeleteAfter != "" && u.DeleteAfter < store.Now() {
		apiError(c, http.StatusGone, "expired", "冷静期已结束，无法恢复")
		return
	}
	if err := s.Store.RestoreUser(u.ID); err != nil {
		apiError(c, http.StatusInternalServerError, "restore_failed", "恢复失败")
		return
	}
	s.audit(c, "user", "restore", u.Username, "恢复冷静期用户", "ok")
	ok(c, gin.H{"message": "用户已恢复"})
}

// deletedUsers lists accounts in the cooling period (Pro only).
func (s *Server) deletedUsers(c *gin.Context) {
	if !s.Pro() {
		ok(c, []store.User{})
		return
	}
	users, err := s.Store.ListDeletedUsers()
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取失败")
		return
	}
	ok(c, users)
}

// resetUserPassword sets a new password for an account.
func (s *Server) resetUserPassword(c *gin.Context) {
	id := int64Param(c, "id")
	u, err := s.Store.GetUserByID(id)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "用户不存在")
		return
	}
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		apiError(c, http.StatusBadRequest, "weak_password", err.Error())
		return
	}
	if err := s.Store.UpdatePassword(u.ID, hash); err != nil {
		apiError(c, http.StatusInternalServerError, "update_failed", "重置密码失败")
		return
	}
	_ = s.Store.RevokeUserSessions(u.ID)
	_ = s.System.SetLinuxPassword(c.Request.Context(), u.Username, req.Password)
	s.audit(c, "user", "reset_password", u.Username, "管理员重置密码", "ok")
	ok(c, gin.H{"message": "密码已重置，该用户需要重新登录"})
}

// unlockUser clears lockout state and failure counters.
func (s *Server) unlockUser(c *gin.Context) {
	id := int64Param(c, "id")
	u, err := s.Store.GetUserByID(id)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "用户不存在")
		return
	}
	_ = s.Store.Unlock(u.ID)
	_ = s.Store.ClearLoginAttempts(u.Username)
	s.audit(c, "user", "unlock", u.Username, "解锁账号", "ok")
	ok(c, gin.H{"message": "账号已解锁"})
}

// userUsage returns quota usage for an account.
func (s *Server) userUsage(c *gin.Context) {
	id := int64Param(c, "id")
	viewer := currentUser(c)
	if s.Pro() && viewer.ID != id {
		apiError(c, http.StatusForbidden, "quota_private", "专业版不允许查看其他用户的配额")
		return
	}
	u, err := s.Store.GetUserByID(id)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "用户不存在")
		return
	}
	usage, err := s.Files.Usage(u)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "usage_failed", "读取用量失败")
		return
	}
	ok(c, gin.H{"user_id": id, "username": u.Username, "usage": usage})
}

// fileUsers lists home directories for admin browsing (Normal edition only).
func (s *Server) fileUsers(c *gin.Context) {
	u := currentUser(c)
	if !s.App.CanSeeOtherFiles(u) {
		ok(c, []string{u.Username})
		return
	}
	users, err := s.Store.ListUsers("", false)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取用户失败")
		return
	}
	out := make([]gin.H, 0, len(users))
	for _, item := range users {
		out = append(out, gin.H{"username": item.Username, "display_name": item.DisplayName, "id": item.ID})
	}
	ok(c, out)
}
