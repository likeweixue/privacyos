package api

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"privacyos/backend/internal/files"
	"privacyos/backend/internal/store"
)

// scopeFor resolves the sandbox scope for a request.
func (s *Server) scopeFor(c *gin.Context) (files.Scope, bool) {
	u := currentUser(c)
	sc := files.Scope{User: u}
	if c.Query("scope") == "all" && s.App.CanSeeOtherFiles(u) {
		sc.SeeAll = true
	}
	if target := c.Query("user"); target != "" && target != u.Username && s.App.CanSeeOtherFiles(u) {
		other, err := s.Store.GetUserByUsername(target)
		if err != nil {
			return sc, false
		}
		sc = files.Scope{User: other}
	}
	return sc, true
}

// listFiles returns a directory listing.
func (s *Server) listFiles(c *gin.Context) {
	sc, good := s.scopeFor(c)
	if !good {
		apiError(c, http.StatusNotFound, "user_not_found", "目标用户不存在")
		return
	}
	listing, err := s.Files.List(sc, c.Query("path"))
	if err != nil {
		s.fileError(c, err)
		return
	}
	ok(c, gin.H{
		"listing": listing, "scope": map[string]bool{"see_all": sc.SeeAll},
		"owner": sc.User.Username, "can_see_all": s.App.CanSeeOtherFiles(currentUser(c)),
	})
}

// statFile returns metadata for one path.
func (s *Server) statFile(c *gin.Context) {
	sc, good := s.scopeFor(c)
	if !good {
		apiError(c, http.StatusNotFound, "user_not_found", "目标用户不存在")
		return
	}
	entry, err := s.Files.Stat(sc, c.Query("path"))
	if err != nil {
		s.fileError(c, err)
		return
	}
	ok(c, entry)
}

// mkdir creates a folder.
func (s *Server) mkdir(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
		Name string `json:"name" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	sc, _ := s.scopeFor(c)
	entry, err := s.Files.Mkdir(sc, req.Path, req.Name)
	if err != nil {
		s.fileError(c, err)
		return
	}
	s.audit(c, "file", "mkdir", entry.Path, "新建文件夹", "ok")
	created(c, entry)
}

// rename renames a file or folder.
func (s *Server) rename(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
		Name string `json:"name" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	sc, _ := s.scopeFor(c)
	if err := s.Files.Rename(sc, req.Path, req.Name); err != nil {
		s.fileError(c, err)
		return
	}
	s.audit(c, "file", "rename", req.Path, "重命名为 "+req.Name, "ok")
	ok(c, gin.H{"message": "重命名成功"})
}

// move moves files into a folder.
func (s *Server) move(c *gin.Context) {
	var req struct {
		Paths       []string `json:"paths" binding:"required"`
		Destination string   `json:"destination"`
	}
	if !bindJSON(c, &req) {
		return
	}
	sc, _ := s.scopeFor(c)
	if err := s.Files.Move(sc, req.Paths, req.Destination); err != nil {
		s.fileError(c, err)
		return
	}
	s.audit(c, "file", "move", strings.Join(req.Paths, ","), "移动到 "+req.Destination, "ok")
	ok(c, gin.H{"message": "移动成功"})
}

// copy duplicates files into a folder.
func (s *Server) copy(c *gin.Context) {
	var req struct {
		Paths       []string `json:"paths" binding:"required"`
		Destination string   `json:"destination"`
	}
	if !bindJSON(c, &req) {
		return
	}
	sc, _ := s.scopeFor(c)
	if err := s.Files.Copy(sc, req.Paths, req.Destination); err != nil {
		s.fileError(c, err)
		return
	}
	s.audit(c, "file", "copy", strings.Join(req.Paths, ","), "复制到 "+req.Destination, "ok")
	ok(c, gin.H{"message": "复制成功"})
}

// deleteFiles moves files to the trash.
func (s *Server) deleteFiles(c *gin.Context) {
	var req struct {
		Paths []string `json:"paths" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	sc, _ := s.scopeFor(c)
	settings := s.App.LoadSettings()
	items, err := s.Files.Delete(sc, req.Paths, files.DeleteOptions{RetentionDays: settings.Deletion.TrashRetentionDays})
	if err != nil {
		s.fileError(c, err)
		return
	}
	s.audit(c, "file", "delete", strings.Join(req.Paths, ","), "移入回收站", "ok")
	ok(c, gin.H{"items": items, "message": "已移入回收站"})
}

// upload streams one or more uploaded files into a directory.
func (s *Server) upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<30)
	form, err := c.MultipartForm()
	if err != nil {
		apiError(c, http.StatusBadRequest, "upload_failed", "解析上传内容失败: "+err.Error())
		return
	}
	sc, _ := s.scopeFor(c)
	dir := c.PostForm("path")
	fileHeaders := form.File["files"]
	if len(fileHeaders) == 0 {
		fileHeaders = form.File["file"]
	}
	if len(fileHeaders) == 0 {
		apiError(c, http.StatusBadRequest, "no_file", "没有收到文件")
		return
	}
	saved := make([]files.Entry, 0, len(fileHeaders))
	for _, fh := range fileHeaders {
		f, err := fh.Open()
		if err != nil {
			apiError(c, http.StatusBadRequest, "upload_failed", "读取上传文件失败")
			return
		}
		entry, err := s.Files.SaveUpload(sc, dir, filepath.Base(fh.Filename), f, fh.Size)
		f.Close()
		if err != nil {
			s.fileError(c, err)
			return
		}
		saved = append(saved, *entry)
		s.audit(c, "file", "upload", entry.Path, fmt.Sprintf("上传文件 %s", entry.Name), "ok")
	}
	ok(c, saved)
}

// download streams a file (Range supported).
func (s *Server) download(c *gin.Context) {
	sc, _ := s.scopeFor(c)
	rel := c.Query("path")
	abs, err := s.Files.Resolve(sc, rel)
	if err != nil {
		s.fileError(c, err)
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "文件不存在")
		return
	}
	if info.IsDir() {
		apiError(c, http.StatusBadRequest, "is_dir", "请先打包目录再下载")
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "open_failed", "无法打开文件")
		return
	}
	defer f.Close()
	s.setDownloadHeaders(c, filepath.Base(abs))
	http.ServeContent(c.Writer, c.Request, filepath.Base(abs), info.ModTime(), f)
	s.audit(c, "file", "download", rel, "下载文件", "ok")
}

// setDownloadHeaders sets content type and disposition.
func (s *Server) setDownloadHeaders(c *gin.Context, name string) {
	if mt := mime.TypeByExtension(filepath.Ext(name)); mt != "" {
		c.Header("Content-Type", mt)
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		asciiFallback(name), urlEncode(name)))
	c.Header("X-Content-Type-Options", "nosniff")
}

// archiveDownload zips selected paths and streams the archive.
func (s *Server) archiveDownload(c *gin.Context) {
	sc, _ := s.scopeFor(c)
	paths := c.QueryArray("path")
	if len(paths) == 0 {
		paths = strings.Split(c.Query("paths"), ",")
	}
	tmp, name, err := s.Files.CreateZip(sc, paths)
	if err != nil {
		s.fileError(c, err)
		return
	}
	defer os.Remove(tmp)
	c.FileAttachment(tmp, name)
	s.audit(c, "file", "archive", strings.Join(paths, ","), "打包下载", "ok")
}

// extract unpacks an archive.
func (s *Server) extract(c *gin.Context) {
	var req struct {
		Path        string `json:"path" binding:"required"`
		Destination string `json:"destination"`
	}
	if !bindJSON(c, &req) {
		return
	}
	sc, _ := s.scopeFor(c)
	dest, err := s.Files.Extract(sc, req.Path, req.Destination)
	if err != nil {
		s.fileError(c, err)
		return
	}
	s.audit(c, "file", "extract", req.Path, "解压到 "+dest, "ok")
	ok(c, gin.H{"path": dest, "message": "解压完成"})
}

// search searches file names in the scope.
func (s *Server) search(c *gin.Context) {
	sc, _ := s.scopeFor(c)
	results, err := s.Files.Search(sc, c.Query("q"), intParam(c, "limit", 100))
	if err != nil {
		s.fileError(c, err)
		return
	}
	ok(c, results)
}

// fileThumb returns a thumbnail for an image or video.
func (s *Server) fileThumb(c *gin.Context) {
	sc, _ := s.scopeFor(c)
	data, err := s.Files.Thumbnail(sc, c.Query("path"), intParam(c, "size", 240))
	if err != nil {
		apiError(c, http.StatusNotFound, "no_thumb", "无法生成缩略图")
		return
	}
	c.Header("Cache-Control", "private, max-age=3600")
	c.Data(http.StatusOK, "image/jpeg", data)
}

// trashList returns the trash for the caller.
func (s *Server) trashList(c *gin.Context) {
	u := currentUser(c)
	items, err := s.Files.ListTrash(u)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取回收站失败")
		return
	}
	ok(c, items)
}

// trashRestore restores trash items.
func (s *Server) trashRestore(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := s.Files.RestoreTrash(currentUser(c), req.IDs); err != nil {
		s.fileError(c, err)
		return
	}
	s.audit(c, "file", "trash_restore", strings.Join(req.IDs, ","), "从回收站恢复", "ok")
	ok(c, gin.H{"message": "已恢复"})
}

// trashEmpty permanently deletes trash items.
func (s *Server) trashEmpty(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := s.Files.EmptyTrash(currentUser(c), req.IDs); err != nil {
		s.fileError(c, err)
		return
	}
	s.audit(c, "file", "trash_empty", "-", "清空回收站", "ok")
	ok(c, gin.H{"message": "回收站已清理"})
}

// fileUsage returns the caller's quota usage.
func (s *Server) fileUsage(c *gin.Context) {
	usage, err := s.Files.Usage(currentUser(c))
	if err != nil {
		apiError(c, http.StatusInternalServerError, "usage_failed", "读取用量失败")
		return
	}
	ok(c, usage)
}

// fileError maps file service errors to HTTP responses.
func (s *Server) fileError(c *gin.Context, err error) {
	switch {
	case err == files.ErrInvalidPath || strings.Contains(err.Error(), "非法"):
		apiError(c, http.StatusBadRequest, "invalid_path", "非法路径")
	case err == files.ErrNotFound || os.IsNotExist(err):
		apiError(c, http.StatusNotFound, "not_found", "文件不存在")
	case err == files.ErrQuota:
		apiError(c, http.StatusInsufficientStorage, "quota_exceeded", "存储配额不足")
	case err == files.ErrConflict:
		apiError(c, http.StatusConflict, "exists", "目标已存在")
	default:
		apiError(c, http.StatusBadRequest, "file_error", err.Error())
	}
}

// createShare creates a public share link for a file or folder.
func (s *Server) createShare(c *gin.Context) {
	var req struct {
		Path         string `json:"path" binding:"required"`
		Name         string `json:"name"`
		Root         string `json:"root"`
		Password     string `json:"password"`
		ExpiresHours int    `json:"expires_hours"`
		DownloadOnly bool   `json:"download_only"`
		MaxDownloads int64  `json:"max_downloads"`
	}
	if !bindJSON(c, &req) {
		return
	}
	u := currentUser(c)
	if !s.App.FeatureAllowed(u, "share") {
		apiError(c, http.StatusForbidden, "feature_disabled", "当前账号未开通分享功能")
		return
	}
	abs, err := s.shareablePath(u, req.Root, req.Path)
	if err != nil {
		s.fileError(c, err)
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "文件不存在")
		return
	}
	sh := &store.Share{
		Name: firstNonEmptyString(req.Name, filepath.Base(abs)), Path: abs, OwnerID: u.ID,
		Kind: map[bool]string{true: "dir", false: "file"}[info.IsDir()],
		DownloadOnly: true, MaxDownloads: req.MaxDownloads, SizeBytes: info.Size(),
	}
	if req.ExpiresHours > 0 {
		sh.ExpiresAt = nowPlusHours(req.ExpiresHours)
	}
	if req.Password != "" {
		hash, err := hashSharePassword(req.Password)
		if err != nil {
			apiError(c, http.StatusBadRequest, "weak_password", err.Error())
			return
		}
		sh.PasswordHash = hash
	}
	if err := s.Store.CreateShare(sh); err != nil {
		apiError(c, http.StatusInternalServerError, "share_failed", "创建分享失败")
		return
	}
	sh.HasPassword = sh.PasswordHash != ""
	s.audit(c, "file", "share", req.Path, "创建分享链接", "ok")
	created(c, gin.H{"share": sh, "url": "/s/" + sh.Token})
}

// shareablePath validates a path for sharing across the allowed roots.
func (s *Server) shareablePath(u *store.User, root, rel string) (string, error) {
	rel = strings.TrimPrefix(rel, "/")
	switch root {
	case "", "home":
		return s.Files.Resolve(files.Scope{User: u}, rel)
	case "photos":
		if !u.Permissions.Photos {
			return "", fmt.Errorf("非法路径")
		}
		return files.SecureJoin(s.Photos.Root(u), rel)
	case "movies":
		if !u.Permissions.Movies {
			return "", fmt.Errorf("非法路径")
		}
		return files.SecureJoin(s.Movies.Root(u), rel)
	default:
		return "", fmt.Errorf("非法路径")
	}
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
