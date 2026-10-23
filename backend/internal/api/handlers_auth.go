package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"privacyos/backend/internal/auth"
	"privacyos/backend/internal/core"
	"privacyos/backend/internal/store"
)

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	TOTP     string `json:"totp"`
}

// login authenticates a user and issues a token pair.
func (s *Server) login(c *gin.Context) {
	var req loginRequest
	if !bindJSON(c, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	ip := clientIP(c)

	gate := s.App.LoginGateFor(req.Username)
	if !gate.Allowed {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": gin.H{"code": "too_many_attempts", "message": rateLimitMessage(gate)},
			"retry_after": gate.RetryAfter, "locked": gate.Locked,
		})
		return
	}

	u, err := s.Store.GetUserByUsername(req.Username)
	if err != nil {
		auth.BurnCompare(req.Password)
		_ = s.Store.RecordAttempt(req.Username, ip, false, "unknown_user")
		s.Store.Audit(&store.AuditLog{Username: req.Username, Category: "auth", Action: "login_failed",
			Detail: "用户不存在", IP: ip, Result: "failed"})
		apiError(c, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
		return
	}
	if u.Status != "active" {
		_ = s.Store.RecordAttempt(req.Username, ip, false, "disabled")
		apiError(c, http.StatusForbidden, "account_disabled", "账号已停用，请联系管理员")
		return
	}
	if !auth.VerifyPassword(u.PasswordHash, req.Password) {
		outcome := s.App.RecordLoginFailure(u, ip)
		s.Store.Audit(&store.AuditLog{UserID: u.ID, Username: u.Username, Category: "auth",
			Action: "login_failed", Detail: "密码错误", IP: ip, Result: "failed"})
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":       gin.H{"code": "invalid_credentials", "message": loginFailureMessage(outcome)},
			"retry_after": outcome.RetryAfter,
			"locked":      outcome.Locked,
			"failures":    outcome.Failures,
		})
		return
	}
	if u.TOTPEnabled {
		if req.TOTP == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": "totp_required", "message": "请输入双重认证验证码"},
			})
			return
		}
		if !auth.VerifyTOTP(u.TOTPSecret, strings.TrimSpace(req.TOTP)) {
			outcome := s.App.RecordLoginFailure(u, ip)
			_ = s.Store.RecordAttempt(u.Username, ip, false, "bad_totp")
			s.Store.Audit(&store.AuditLog{UserID: u.ID, Username: u.Username, Category: "auth",
				Action: "login_failed", Detail: "动态验证码错误", IP: ip, Result: "failed"})
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":       gin.H{"code": "invalid_totp", "message": loginFailureMessage(outcome)},
				"retry_after": outcome.RetryAfter,
				"locked":      outcome.Locked,
				"failures":    outcome.Failures,
			})
			return
		}
	}

	session := &store.Session{
		ID: store.NewID(), UserID: u.ID, IP: ip, UserAgent: c.Request.UserAgent(),
		CreatedAt: store.Now(), LastSeen: store.Now(), ExpiresAt: s.Tokens.SessionExpiry(),
	}
	if err := s.Store.CreateSession(session); err != nil {
		apiError(c, http.StatusInternalServerError, "session_error", "创建登录会话失败")
		return
	}
	pair, err := s.Tokens.Issue(u, session.ID)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "token_error", "签发登录凭证失败")
		return
	}
	_ = s.Store.TouchLogin(u.ID, ip)
	_ = s.Store.RecordAttempt(u.Username, ip, true, "password")
	s.Store.Audit(&store.AuditLog{UserID: u.ID, Username: u.Username, Category: "auth",
		Action: "login", Detail: "登录成功", IP: ip, Result: "ok"})
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"access_token": pair.AccessToken, "refresh_token": pair.RefreshToken,
		"expires_in": pair.ExpiresIn, "user": u,
	}})
}

func rateLimitMessage(gate core.LoginGate) string {
	if gate.Locked {
		return "账号已锁定，请在 " + humanSeconds(gate.RetryAfter) + "后重试"
	}
	return "操作过于频繁，请在 " + humanSeconds(gate.RetryAfter) + "后重试"
}

func loginFailureMessage(outcome core.LoginOutcome) string {
	if outcome.Locked {
		if outcome.RetryAfter > 0 {
			return "密码错误次数过多，账号已临时锁定 " + humanSeconds(outcome.RetryAfter)
		}
		return "密码错误次数过多，账号已锁定"
	}
	if outcome.RetryAfter > 0 {
		return "用户名或密码错误，请在 " + humanSeconds(outcome.RetryAfter) + "后重试"
	}
	return "用户名或密码错误"
}

func humanSeconds(sec int) string {
	if sec <= 0 {
		return "片刻"
	}
	return (time.Duration(sec) * time.Second).String()
}

// refresh rotates tokens for a valid refresh token.
func (s *Server) refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	claims, err := s.Tokens.Parse(req.RefreshToken)
	if err != nil || claims.Type != "refresh" {
		apiError(c, http.StatusUnauthorized, "invalid_refresh", "刷新凭证无效，请重新登录")
		return
	}
	sess, err := s.Store.GetSession(claims.ID)
	if err != nil || sess.RevokedAt != "" || store.Time(sess.ExpiresAt).Before(time.Now()) {
		apiError(c, http.StatusUnauthorized, "invalid_refresh", "登录会话已失效，请重新登录")
		return
	}
	u, err := s.Store.GetUserByID(claims.UserID)
	if err != nil || u.Status != "active" {
		apiError(c, http.StatusUnauthorized, "invalid_refresh", "账号不可用")
		return
	}
	pair, err := s.Tokens.Issue(u, sess.ID)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "token_error", "签发凭证失败")
		return
	}
	_ = s.Store.TouchLogin(u.ID, clientIP(c))
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"access_token": pair.AccessToken, "refresh_token": pair.RefreshToken,
		"expires_in": pair.ExpiresIn, "user": u,
	}})
}

// me returns the current account and edition capabilities.
func (s *Server) me(c *gin.Context) {
	u := currentUser(c)
	st := s.App.LoadSettings()
	ok(c, gin.H{
		"user": u, "edition": s.Cfg.Edition, "edition_name": s.Cfg.EditionName(),
		"pro": s.Cfg.Pro(), "version": s.Cfg.Version,
		"admin_see_all": s.App.AdminSeeAll(), "dev": s.Cfg.Dev,
		"brand": gin.H{"name": st.System.Name, "website": st.System.Website},
		"features": gin.H{
			"docker": u.Permissions.Docker, "apps": u.Permissions.Apps,
			"photos": u.Permissions.Photos, "movies": u.Permissions.Movies,
			"files": u.Permissions.Files, "share": u.Permissions.Share,
		},
	})
}

// logout revokes the current session.
func (s *Server) logout(c *gin.Context) {
	sid := currentSessionID(c)
	if sid != "" {
		_ = s.Store.RevokeSession(sid)
	}
	s.audit(c, "auth", "logout", "-", "用户退出登录", "ok")
	ok(c, gin.H{"message": "已退出登录"})
}

// changePassword updates the caller's password and revokes other sessions.
func (s *Server) changePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	u := currentUser(c)
	if !auth.VerifyPassword(u.PasswordHash, req.OldPassword) {
		apiError(c, http.StatusBadRequest, "invalid_password", "原密码不正确")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		apiError(c, http.StatusBadRequest, "weak_password", err.Error())
		return
	}
	if err := s.Store.UpdatePassword(u.ID, hash); err != nil {
		apiError(c, http.StatusInternalServerError, "update_failed", "修改密码失败")
		return
	}
	current := currentSessionID(c)
	sessions, _ := s.Store.ListUserSessions(u.ID)
	for _, sess := range sessions {
		if sess.ID != current {
			_ = s.Store.RevokeSession(sess.ID)
		}
	}
	s.audit(c, "auth", "password_change", u.Username, "修改登录密码", "ok")
	ok(c, gin.H{"message": "密码已更新，其他设备已退出登录"})
}

// sessionsList returns the caller's active sessions.
func (s *Server) sessionsList(c *gin.Context) {
	u := currentUser(c)
	sessions, err := s.Store.ListUserSessions(u.ID)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取会话失败")
		return
	}
	current := currentSessionID(c)
	for i := range sessions {
		if sessions[i].ID == current {
			sessions[i].UserAgent += " (当前设备)"
		}
	}
	ok(c, sessions)
}

// sessionRevoke ends one of the caller's sessions.
func (s *Server) sessionRevoke(c *gin.Context) {
	id := c.Param("id")
	if !s.sessionOwnedBy(c, id) {
		apiError(c, http.StatusNotFound, "not_found", "会话不存在")
		return
	}
	_ = s.Store.RevokeSession(id)
	s.audit(c, "auth", "session_revoke", id, "注销登录会话", "ok")
	ok(c, gin.H{"message": "会话已注销"})
}

// totpSetup generates a new TOTP secret (not yet enabled).
func (s *Server) totpSetup(c *gin.Context) {
	u := currentUser(c)
	secret, otpauth, err := auth.NewTOTP(u.Username)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "totp_error", "生成动态口令失败")
		return
	}
	u.TOTPSecret = secret
	u.TOTPEnabled = false
	if err := s.Store.UpdateUser(u); err != nil {
		apiError(c, http.StatusInternalServerError, "totp_error", "保存动态口令失败")
		return
	}
	ok(c, gin.H{"secret": secret, "otpauth_url": otpauth, "issuer": "PrivacyOS", "account": u.Username})
}

// totpEnable verifies a code and enables 2FA.
func (s *Server) totpEnable(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	u := currentUser(c)
	if u.TOTPSecret == "" {
		apiError(c, http.StatusBadRequest, "totp_not_initialized", "请先生成动态口令密钥")
		return
	}
	if !auth.VerifyTOTP(u.TOTPSecret, req.Code) {
		apiError(c, http.StatusBadRequest, "invalid_totp", "验证码不正确")
		return
	}
	u.TOTPEnabled = true
	if err := s.Store.UpdateUser(u); err != nil {
		apiError(c, http.StatusInternalServerError, "totp_error", "启用失败")
		return
	}
	s.audit(c, "auth", "2fa_enable", u.Username, "启用双重认证", "ok")
	ok(c, gin.H{"message": "双重认证已启用"})
}

// totpDisable turns off 2FA after password + code verification.
func (s *Server) totpDisable(c *gin.Context) {
	var req struct {
		Password string `json:"password" binding:"required"`
		Code     string `json:"code" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	u := currentUser(c)
	if !auth.VerifyPassword(u.PasswordHash, req.Password) || !auth.VerifyTOTP(u.TOTPSecret, req.Code) {
		apiError(c, http.StatusBadRequest, "invalid_credentials", "密码或验证码不正确")
		return
	}
	u.TOTPEnabled = false
	u.TOTPSecret = ""
	if err := s.Store.UpdateUser(u); err != nil {
		apiError(c, http.StatusInternalServerError, "totp_error", "关闭失败")
		return
	}
	s.audit(c, "auth", "2fa_disable", u.Username, "关闭双重认证", "ok")
	ok(c, gin.H{"message": "双重认证已关闭"})
}

// updatePreferences stores UI language/theme.
func (s *Server) updatePreferences(c *gin.Context) {
	var req struct {
		Lang  string `json:"lang"`
		Theme string `json:"theme"`
	}
	if !bindJSON(c, &req) {
		return
	}
	u := currentUser(c)
	if req.Lang != "" {
		if req.Lang != "zh-CN" && req.Lang != "en-US" {
			apiError(c, http.StatusBadRequest, "invalid_lang", "不支持的语言")
			return
		}
		u.Preferences.Lang = req.Lang
	}
	if req.Theme != "" {
		if req.Theme != "dark" && req.Theme != "light" {
			apiError(c, http.StatusBadRequest, "invalid_theme", "不支持的主题")
			return
		}
		u.Preferences.Theme = req.Theme
	}
	if err := s.Store.UpdateUser(u); err != nil {
		apiError(c, http.StatusInternalServerError, "update_failed", "保存偏好失败")
		return
	}
	ok(c, gin.H{"preferences": u.Preferences})
}
