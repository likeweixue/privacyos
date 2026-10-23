package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"privacyos/backend/internal/store"
)

// apiError sends a structured error response.
func apiError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

// ok sends a success envelope.
func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// created sends a 201 envelope.
func created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, gin.H{"data": data})
}

// paginated sends a list envelope.
func paginated(c *gin.Context, items any, total, page, size int) {
	c.JSON(http.StatusOK, gin.H{
		"data": items,
		"pagination": gin.H{"total": total, "page": page, "page_size": size},
	})
}

// bindJSON decodes and validates a JSON body.
func bindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		apiError(c, http.StatusBadRequest, "bad_request", "请求参数不正确: "+err.Error())
		return false
	}
	return true
}

// intParam parses a query parameter with a default.
func intParam(c *gin.Context, key string, def int) int {
	raw := c.Query(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}

// int64Param parses an int64 path/query value.
func int64Param(c *gin.Context, key string) int64 {
	v, _ := strconv.ParseInt(c.Param(key), 10, 64)
	return v
}

// clientIP returns the caller address.
func clientIP(c *gin.Context) string {
	ip := c.ClientIP()
	if ip == "" {
		ip = c.Request.RemoteAddr
	}
	if idx := strings.LastIndex(ip, ":"); idx > 0 && strings.Count(ip, ":") == 1 {
		ip = ip[:idx]
	}
	return ip
}

// currentUser returns the authenticated account.
func currentUser(c *gin.Context) *store.User {
	if v, exists := c.Get(ctxUser); exists {
		if u, ok := v.(*store.User); ok {
			return u
		}
	}
	return nil
}

// currentSessionID returns the session bound to the access token.
func currentSessionID(c *gin.Context) string {
	if v, exists := c.Get(ctxSession); exists {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// requireAdmin aborts with 403 when the caller is not an administrator.
func (s *Server) requireAdmin(c *gin.Context) bool {
	if !isAdmin(c) {
		apiError(c, http.StatusForbidden, "forbidden", "需要管理员权限")
		return false
	}
	return true
}

// isAdmin reports whether the caller is an administrator.
func isAdmin(c *gin.Context) bool {
	u := currentUser(c)
	return u != nil && u.Role == "admin"
}

// pageParams returns page/page_size with sane bounds.
func pageParams(c *gin.Context, defSize int) (page, size int) {
	page = intParam(c, "page", 1)
	size = intParam(c, "page_size", defSize)
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 500 {
		size = defSize
	}
	return page, size
}
