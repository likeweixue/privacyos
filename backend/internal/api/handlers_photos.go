package api

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"privacyos/backend/internal/media"
	"privacyos/backend/internal/store"
)

// listPhotos returns a filtered photo page.
func (s *Server) listPhotos(c *gin.Context) {
	u := currentUser(c)
	page, size := pageParams(c, 60)
	var photos []store.Photo
	var total int
	var err error
	if albumID := intParam(c, "album", 0); albumID > 0 {
		photos, err = s.Store.AlbumPhotos(int64(albumID), u.ID)
		total = len(photos)
		if err == nil && (page-1)*size < len(photos) {
			end := page * size
			if end > len(photos) {
				end = len(photos)
			}
			photos = photos[(page-1)*size : end]
		} else if err == nil {
			photos = []store.Photo{}
		}
	} else {
		photos, total, err = s.Store.ListPhotos(u.ID, c.DefaultQuery("view", ""), c.Query("q"), c.Query("from"), c.Query("to"), page, size)
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取照片失败")
		return
	}
	for i := range photos {
		media.Decorate(&photos[i])
	}
	paginated(c, photos, total, page, size)
}

// photoTimeline returns year/month/day aggregation.
func (s *Server) photoTimeline(c *gin.Context) {
	groups, err := s.Photos.Timeline(currentUser(c))
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取时间轴失败")
		return
	}
	ok(c, groups)
}

// photoMap returns geo-tagged photos.
func (s *Server) photoMap(c *gin.Context) {
	photos, err := s.Store.PhotosWithGPS(currentUser(c).ID)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取照片失败")
		return
	}
	for i := range photos {
		media.Decorate(&photos[i])
	}
	ok(c, photos)
}

// getPhoto returns one photo with EXIF details.
func (s *Server) getPhoto(c *gin.Context) {
	p, err := s.Store.GetPhoto(int64Param(c, "id"), currentUser(c).ID)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "照片不存在")
		return
	}
	media.Decorate(p)
	ok(c, p)
}

// photoFile streams the original image (Range supported).
func (s *Server) photoFile(c *gin.Context) {
	p, err := s.Store.GetPhoto(int64Param(c, "id"), currentUser(c).ID)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "照片不存在")
		return
	}
	f, err := os.Open(p.Path)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "文件已丢失")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		apiError(c, http.StatusInternalServerError, "stat_failed", "读取失败")
		return
	}
	if mt := mime.TypeByExtension(filepath.Ext(p.Path)); mt != "" {
		c.Header("Content-Type", mt)
	}
	c.Header("Cache-Control", "private, max-age=86400")
	http.ServeContent(c.Writer, c.Request, p.FileName, info.ModTime(), f)
}

// photoThumb returns a JPEG thumbnail.
func (s *Server) photoThumb(c *gin.Context) {
	p, err := s.Store.GetPhoto(int64Param(c, "id"), currentUser(c).ID)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "照片不存在")
		return
	}
	data, err := s.Photos.Thumb(p, intParam(c, "size", 400))
	if err != nil {
		apiError(c, http.StatusNotFound, "no_thumb", "无法生成缩略图")
		return
	}
	c.Header("Cache-Control", "private, max-age=86400")
	c.Data(http.StatusOK, "image/jpeg", data)
}

// photoUpload stores uploaded images into the date archive.
func (s *Server) photoUpload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8<<30)
	form, err := c.MultipartForm()
	if err != nil {
		apiError(c, http.StatusBadRequest, "upload_failed", "解析上传内容失败")
		return
	}
	u := currentUser(c)
	headers := form.File["files"]
	if len(headers) == 0 {
		headers = form.File["file"]
	}
	if len(headers) == 0 {
		apiError(c, http.StatusBadRequest, "no_file", "没有收到图片")
		return
	}
	usage, _ := s.Files.Usage(u)
	var added int64
	for _, fh := range headers {
		added += fh.Size
	}
	if u.QuotaBytes > 0 && usage.UsedBytes+added > u.QuotaBytes {
		apiError(c, http.StatusInsufficientStorage, "quota_exceeded", "存储配额不足")
		return
	}
	photos := make([]store.Photo, 0, len(headers))
	for _, fh := range headers {
		f, err := fh.Open()
		if err != nil {
			apiError(c, http.StatusBadRequest, "upload_failed", "读取图片失败")
			return
		}
		p, err := s.Photos.SaveUpload(u, fh.Filename, f, time.Now())
		f.Close()
		if err != nil {
			apiError(c, http.StatusInternalServerError, "upload_failed", "保存图片失败: "+err.Error())
			return
		}
		media.Decorate(p)
		photos = append(photos, *p)
	}
	s.Files.Invalidate(u.ID)
	s.audit(c, "photo", "upload", fmt.Sprintf("%d 张", len(photos)), "上传照片", "ok")
	created(c, photos)
}

// photoScan starts a background library scan.
func (s *Server) photoScan(c *gin.Context) {
	u := currentUser(c)
	job := s.App.Jobs.Start("扫描相册", func(ctx context.Context, report func(int, string)) error {
		root := s.Photos.Root(u)
		if err := os.MkdirAll(root, 0o700); err != nil {
			return err
		}
		n, err := s.Photos.Scan(ctx, u, report)
		if err == nil {
			report(100, fmt.Sprintf("扫描完成，索引 %d 张照片", n))
		}
		s.Files.Invalidate(u.ID)
		return err
	})
	created(c, job)
}

// photoDelete soft-deletes a photo (restorable from the Deleted view).
func (s *Server) photoDelete(c *gin.Context) {
	u := currentUser(c)
	if err := s.Photos.SoftDelete(u, int64Param(c, "id")); err != nil {
		apiError(c, http.StatusBadRequest, "delete_failed", err.Error())
		return
	}
	s.Files.Invalidate(u.ID)
	s.audit(c, "photo", "delete", c.Param("id"), "移入已删除", "ok")
	ok(c, gin.H{"message": "照片已移入已删除"})
}

// photoRestore restores a soft-deleted photo.
func (s *Server) photoRestore(c *gin.Context) {
	u := currentUser(c)
	if err := s.Photos.Restore(u, int64Param(c, "id")); err != nil {
		apiError(c, http.StatusBadRequest, "restore_failed", err.Error())
		return
	}
	s.Files.Invalidate(u.ID)
	s.audit(c, "photo", "restore", c.Param("id"), "恢复照片", "ok")
	ok(c, gin.H{"message": "照片已恢复"})
}

// photoPurge permanently removes a photo file and index row.
func (s *Server) photoPurge(c *gin.Context) {
	u := currentUser(c)
	if err := s.Photos.Delete(u, int64Param(c, "id")); err != nil {
		apiError(c, http.StatusBadRequest, "delete_failed", err.Error())
		return
	}
	s.Files.Invalidate(u.ID)
	s.audit(c, "photo", "purge", c.Param("id"), "永久删除照片", "ok")
	ok(c, gin.H{"message": "照片已永久删除"})
}

// photoFavorite toggles the favorite flag.
func (s *Server) photoFavorite(c *gin.Context) {
	var req struct {
		Favorite bool `json:"favorite"`
	}
	_ = c.ShouldBindJSON(&req)
	u := currentUser(c)
	if err := s.Store.SetPhotoFavorite(int64Param(c, "id"), u.ID, req.Favorite); err != nil {
		apiError(c, http.StatusBadRequest, "favorite_failed", err.Error())
		return
	}
	ok(c, gin.H{"favorite": req.Favorite})
}

// photoSourcesList returns the extra photo source folders.
func (s *Server) photoSourcesList(c *gin.Context) {
	sources, err := s.Photos.Sources(currentUser(c))
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取照片来源失败")
		return
	}
	ok(c, sources)
}

// photoSourcesAdd registers one or more folders and indexes their images.
func (s *Server) photoSourcesAdd(c *gin.Context) {
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Paths) == 0 {
		apiError(c, http.StatusBadRequest, "invalid_source", "请选择文件夹")
		return
	}
	u := currentUser(c)
	total := 0
	added := make([]store.PhotoSource, 0, len(req.Paths))
	for _, rel := range req.Paths {
		src, n, err := s.Photos.AddSource(u, rel)
		if err != nil {
			apiError(c, http.StatusBadRequest, "invalid_source", err.Error())
			return
		}
		total += n
		added = append(added, *src)
	}
	s.Files.Invalidate(u.ID)
	s.audit(c, "photo", "source_add", fmt.Sprintf("%d folders", len(added)), "添加照片来源", "ok")
	created(c, gin.H{"sources": added, "indexed": total})
}

// photoSourcesDelete removes a registered source folder.
func (s *Server) photoSourcesDelete(c *gin.Context) {
	u := currentUser(c)
	if err := s.Photos.DeleteSource(u, int64Param(c, "id")); err != nil {
		apiError(c, http.StatusBadRequest, "delete_failed", err.Error())
		return
	}
	ok(c, gin.H{"message": "已移除照片来源"})
}

// photoDuplicates returns groups of identical photos by SHA-256.
func (s *Server) photoDuplicates(c *gin.Context) {
	groups, err := s.Store.DuplicateGroups(currentUser(c).ID)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取重复项目失败")
		return
	}
	for gi := range groups {
		for pi := range groups[gi].Photos {
			media.Decorate(&groups[gi].Photos[pi])
		}
	}
	ok(c, groups)
}

// photoDuplicatesMerge keeps the best copy in every duplicate group.
func (s *Server) photoDuplicatesMerge(c *gin.Context) {
	u := currentUser(c)
	removed, err := s.Store.MergeDuplicateGroups(u.ID)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "merge_failed", err.Error())
		return
	}
	s.Files.Invalidate(u.ID)
	s.audit(c, "photo", "dedupe", fmt.Sprintf("%d", removed), "合并重复照片", "ok")
	ok(c, gin.H{"removed": removed})
}

// photoShare creates a public link for a photo.
func (s *Server) photoShare(c *gin.Context) {
	u := currentUser(c)
	p, err := s.Store.GetPhoto(int64Param(c, "id"), u.ID)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "照片不存在")
		return
	}
	if !s.App.FeatureAllowed(u, "share") {
		apiError(c, http.StatusForbidden, "feature_disabled", "当前账号未开通分享功能")
		return
	}
	var req struct {
		Password     string `json:"password"`
		ExpiresHours int    `json:"expires_hours"`
	}
	_ = c.ShouldBindJSON(&req)
	sh := &store.Share{
		Name: p.FileName, Path: p.Path, OwnerID: u.ID, Kind: "file",
		DownloadOnly: true, SizeBytes: p.SizeBytes,
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
	s.audit(c, "photo", "share", p.FileName, "创建照片分享", "ok")
	created(c, gin.H{"share": sh, "url": "/s/" + sh.Token})
}

// ---------- albums ----------

// albumsList returns the caller's albums.
func (s *Server) albumsList(c *gin.Context) {
	albums, err := s.Store.ListAlbums(currentUser(c).ID)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取相册失败")
		return
	}
	ok(c, albums)
}

// albumCreate creates an album.
func (s *Server) albumCreate(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if !bindJSON(c, &req) {
		return
	}
	a := &store.Album{OwnerID: currentUser(c).ID, Name: strings.TrimSpace(req.Name), Description: req.Description}
	if a.Name == "" {
		apiError(c, http.StatusBadRequest, "invalid_name", "相册名称不能为空")
		return
	}
	if err := s.Store.CreateAlbum(a); err != nil {
		apiError(c, http.StatusBadRequest, "create_failed", "创建相册失败（名称可能重复）")
		return
	}
	s.audit(c, "photo", "album_create", a.Name, "创建相册", "ok")
	created(c, a)
}

// albumGet returns album contents.
func (s *Server) albumGet(c *gin.Context) {
	u := currentUser(c)
	id := int64Param(c, "id")
	a, err := s.Store.GetAlbum(id, u.ID)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "相册不存在")
		return
	}
	photos, err := s.Store.AlbumPhotos(id, u.ID)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取相册失败")
		return
	}
	for i := range photos {
		media.Decorate(&photos[i])
	}
	ok(c, gin.H{"album": a, "photos": photos})
}

// albumDelete removes an album (photos are kept).
func (s *Server) albumDelete(c *gin.Context) {
	u := currentUser(c)
	if err := s.Store.DeleteAlbum(int64Param(c, "id"), u.ID); err != nil {
		apiError(c, http.StatusInternalServerError, "delete_failed", "删除相册失败")
		return
	}
	s.audit(c, "photo", "album_delete", c.Param("id"), "删除相册", "ok")
	ok(c, gin.H{"message": "相册已删除"})
}

// albumAddPhotos links photos into an album.
func (s *Server) albumAddPhotos(c *gin.Context) {
	var req struct {
		PhotoIDs []int64 `json:"photo_ids" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	u := currentUser(c)
	id := int64Param(c, "id")
	if _, err := s.Store.GetAlbum(id, u.ID); err != nil {
		apiError(c, http.StatusNotFound, "not_found", "相册不存在")
		return
	}
	if err := s.Store.AddPhotosToAlbum(id, u.ID, req.PhotoIDs); err != nil {
		apiError(c, http.StatusInternalServerError, "update_failed", "添加失败")
		return
	}
	ok(c, gin.H{"message": "已加入相册"})
}

// albumRemovePhoto removes a photo from an album.
func (s *Server) albumRemovePhoto(c *gin.Context) {
	u := currentUser(c)
	albumID := int64Param(c, "id")
	photoID, _ := strconv.ParseInt(c.Param("photoId"), 10, 64)
	if err := s.Store.RemovePhotoFromAlbum(albumID, u.ID, photoID); err != nil {
		apiError(c, http.StatusInternalServerError, "update_failed", "移除失败")
		return
	}
	ok(c, gin.H{"message": "已从相册移除"})
}
