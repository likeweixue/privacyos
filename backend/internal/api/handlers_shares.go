package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"privacyos/backend/internal/store"
)

// listShares returns share links for the caller (admins see everything).
func (s *Server) listShares(c *gin.Context) {
	u := currentUser(c)
	all := false
	if u.Role == "admin" && c.Query("all") == "1" {
		all = true
	}
	shares, err := s.Store.ListShares(u.ID, all)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取分享失败")
		return
	}
	for i := range shares {
		shares[i].URL = "/s/" + shares[i].Token
		shares[i].Path = maskPath(shares[i].Path)
	}
	ok(c, shares)
}

// getShare returns a single share link.
func (s *Server) getShare(c *gin.Context) {
	sh, err := s.Store.GetShare(c.Param("id"))
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "分享不存在")
		return
	}
	u := currentUser(c)
	if sh.OwnerID != u.ID && u.Role != "admin" {
		apiError(c, http.StatusForbidden, "forbidden", "无权查看")
		return
	}
	sh.URL = "/s/" + sh.Token
	ok(c, sh)
}

// deleteShare revokes a share link.
func (s *Server) deleteShare(c *gin.Context) {
	sh, err := s.Store.GetShare(c.Param("id"))
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "分享不存在")
		return
	}
	u := currentUser(c)
	if sh.OwnerID != u.ID && u.Role != "admin" {
		apiError(c, http.StatusForbidden, "forbidden", "无权删除")
		return
	}
	if err := s.Store.DeleteShare(sh.ID); err != nil {
		apiError(c, http.StatusInternalServerError, "delete_failed", "删除分享失败")
		return
	}
	s.audit(c, "file", "share_delete", sh.Name, "撤销分享链接", "ok")
	ok(c, gin.H{"message": "分享已撤销"})
}

// publicShare returns a share by token or writes an error.
func (s *Server) publicShare(c *gin.Context) (*store.Share, bool) {
	sh, err := s.Store.GetShareByToken(c.Param("token"))
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "分享链接无效")
		return nil, false
	}
	if sh.ExpiresAt != "" && store.Time(sh.ExpiresAt).Before(time.Now()) {
		apiError(c, http.StatusGone, "expired", "分享链接已过期")
		return nil, false
	}
	if sh.MaxDownloads > 0 && sh.Downloads >= sh.MaxDownloads {
		apiError(c, http.StatusGone, "limit_reached", "分享下载次数已达上限")
		return nil, false
	}
	return sh, true
}

// publicShareInfo returns metadata for the public share page.
func (s *Server) publicShareInfo(c *gin.Context) {
	sh, good := s.publicShare(c)
	if !good {
		return
	}
	authorized := !sh.HasPassword || s.checkShareGrant(c, sh.Token)
	info := gin.H{
		"name": sh.Name, "kind": sh.Kind, "size": sh.SizeBytes,
		"has_password": sh.HasPassword, "authorized": authorized,
		"expires_at": sh.ExpiresAt, "downloads": sh.Downloads,
		"max_downloads": sh.MaxDownloads, "download_only": sh.DownloadOnly,
		"created_at": sh.CreatedAt,
	}
	if authorized {
		info["download_url"] = "/api/v1/s/" + sh.Token + "/download"
	}
	c.JSON(http.StatusOK, gin.H{"data": info})
}

// publicShareVerify checks a share password and sets a grant cookie.
func (s *Server) publicShareVerify(c *gin.Context) {
	sh, good := s.publicShare(c)
	if !good {
		return
	}
	if !sh.HasPassword {
		ok(c, gin.H{"authorized": true})
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(sh.PasswordHash), []byte(req.Password)) != nil {
		time.Sleep(250 * time.Millisecond)
		apiError(c, http.StatusUnauthorized, "invalid_password", "分享密码不正确")
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(shareCookie, s.makeShareGrant(sh.Token), 7200, "/api/v1/s/"+sh.Token, "", c.Request.TLS != nil, true)
	ok(c, gin.H{"authorized": true, "download_url": "/api/v1/s/" + sh.Token + "/download"})
}

// publicShareDownload streams the shared file or a zip of the shared folder.
func (s *Server) publicShareDownload(c *gin.Context) {
	sh, good := s.publicShare(c)
	if !good {
		return
	}
	if sh.HasPassword && !s.checkShareGrant(c, sh.Token) {
		apiError(c, http.StatusUnauthorized, "password_required", "需要分享密码")
		return
	}
	info, err := os.Stat(sh.Path)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "文件已不存在")
		return
	}
	_ = s.Store.TouchShare(sh.ID, true)
	if info.IsDir() {
		tmp, err := s.Files.ZipAbsolute([]string{sh.Path}, sh.Name+".zip")
		if err != nil {
			apiError(c, http.StatusInternalServerError, "archive_failed", "打包失败")
			return
		}
		defer os.Remove(tmp)
		c.FileAttachment(tmp, sh.Name+".zip")
		return
	}
	f, err := os.Open(sh.Path)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "open_failed", "无法打开文件")
		return
	}
	defer f.Close()
	s.setDownloadHeaders(c, filepath.Base(sh.Path))
	http.ServeContent(c.Writer, c.Request, filepath.Base(sh.Path), info.ModTime(), f)
}

// maskPath shortens an absolute path for list responses.
func maskPath(p string) string {
	idx := strings.LastIndex(p, "/")
	if idx > 0 {
		return "…/" + p[idx+1:]
	}
	return p
}
