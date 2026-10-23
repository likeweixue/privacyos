package api

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"privacyos/backend/internal/auth"
	"privacyos/backend/internal/store"
)

// tokenFromRequest extracts a bearer token from the header or query string.
func tokenFromRequest(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	}
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
		return c.Query("access_token")
	}
	return ""
}

// authRequired validates the access token and loads the account.
func (s *Server) authRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := tokenFromRequest(c)
		if raw == "" {
			apiError(c, http.StatusUnauthorized, "unauthorized", "请先登录")
			return
		}
		claims, err := s.Tokens.Parse(raw)
		if err != nil {
			apiError(c, http.StatusUnauthorized, "unauthorized", "登录状态已失效，请重新登录")
			return
		}
		if claims.Type != "access" {
			apiError(c, http.StatusUnauthorized, "unauthorized", "凭证类型不正确")
			return
		}
		u, err := s.Store.GetUserByID(claims.UserID)
		if err != nil {
			apiError(c, http.StatusUnauthorized, "unauthorized", "账号不存在")
			return
		}
		if u.Status != "active" {
			apiError(c, http.StatusForbidden, "account_disabled", "账号已停用")
			return
		}
		sess, err := s.Store.GetSession(claims.ID)
		if err != nil || sess.RevokedAt != "" {
			apiError(c, http.StatusUnauthorized, "unauthorized", "登录会话已注销")
			return
		}
		if store.Time(sess.ExpiresAt).Before(time.Now()) {
			apiError(c, http.StatusUnauthorized, "unauthorized", "登录会话已过期")
			return
		}
		if lastSeen := store.Time(sess.LastSeen); time.Since(lastSeen) > 5*time.Minute {
			s.Store.TouchSession(sess.ID)
		}
		c.Set(ctxUser, u)
		c.Set(ctxSession, sess.ID)
		c.Set(ctxClaims, claims)
		c.Next()
	}
}

// adminOnly restricts a route to administrators.
func (s *Server) adminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := currentUser(c)
		if u == nil || u.Role != "admin" {
			apiError(c, http.StatusForbidden, "forbidden", "需要管理员权限")
			return
		}
		c.Next()
	}
}

// feature enforces a per-user feature permission (admins bypass).
func (s *Server) feature(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := currentUser(c)
		if u == nil {
			apiError(c, http.StatusUnauthorized, "unauthorized", "请先登录")
			return
		}
		if u.Role == "admin" {
			c.Next()
			return
		}
		if !s.App.FeatureAllowed(u, name) {
			apiError(c, http.StatusForbidden, "feature_disabled", "当前账号未开通该功能")
			return
		}
		c.Next()
	}
}

// rateLimiter is a tiny sliding-window limiter for authentication attempts.
type rateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
	lastGC time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{hits: map[string][]time.Time{}, limit: limit, window: window, lastGC: time.Now()}
}

func (r *rateLimiter) allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if now.Sub(r.lastGC) > 10*time.Minute {
		for k, v := range r.hits {
			if len(v) == 0 || now.Sub(v[len(v)-1]) > r.window {
				delete(r.hits, k)
			}
		}
		r.lastGC = now
	}
	cutoff := now.Add(-r.window)
	var kept []time.Time
	for _, t := range r.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= r.limit {
		r.hits[key] = kept
		return false
	}
	r.hits[key] = append(kept, now)
	return true
}

// loginLimiter throttles login attempts per IP.
func (s *Server) loginLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.attempts.allow(clientIP(c)) {
			apiError(c, http.StatusTooManyRequests, "rate_limited", "尝试过于频繁，请稍后再试")
			return
		}
		c.Next()
	}
}

// verifySessionOwnership checks that a session belongs to the caller.
func (s *Server) sessionOwnedBy(c *gin.Context, sessionID string) bool {
	sess, err := s.Store.GetSession(sessionID)
	if err != nil {
		return false
	}
	u := currentUser(c)
	return u != nil && sess.UserID == u.ID
}

var _ = auth.ErrInvalidToken
