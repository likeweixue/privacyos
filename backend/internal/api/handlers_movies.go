package api

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"privacyos/backend/internal/media"
	"privacyos/backend/internal/store"
)

// listMovies returns a filtered movie library page.
func (s *Server) listMovies(c *gin.Context) {
	u := currentUser(c)
	page, size := pageParams(c, 40)
	filter := store.MovieFilter{
		OwnerID: u.ID, Query: c.Query("q"), Kind: c.Query("kind"),
		Genre: c.Query("genre"), Sort: c.Query("sort"),
		Page: page, PageSize: size,
	}
	if y := intParam(c, "year", 0); y > 0 {
		filter.Year = y
	}
	movies, total, err := s.Store.ListMovies(filter)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取影视库失败")
		return
	}
	for i := range movies {
		s.Movies.Decorate(&movies[i])
	}
	paginated(c, movies, total, page, size)
}

// movieMeta returns filter facets and TMDB configuration state.
func (s *Server) movieMeta(c *gin.Context) {
	u := currentUser(c)
	years, _ := s.Store.DistinctYears(u.ID)
	kinds, _ := s.Store.CountMoviesByKind(u.ID)
	total, _ := s.Store.CountMovies(u.ID)
	settings := s.App.LoadSettings()
	ok(c, gin.H{
		"years": years, "kinds": kinds, "total": total,
		"tmdb": gin.H{"configured": settings.TMDB.APIKey != "", "language": settings.TMDB.Language},
	})
}

// movieTmdbGet returns TMDB settings (admin only, key masked).
func (s *Server) movieTmdbGet(c *gin.Context) {
	if !isAdmin(c) {
		apiError(c, http.StatusForbidden, "forbidden", "需要管理员权限")
		return
	}
	settings := s.App.LoadSettings()
	ok(c, gin.H{"configured": settings.TMDB.APIKey != "", "language": settings.TMDB.Language,
		"api_key": maskSecret(settings.TMDB.APIKey)})
}

// movieTmdbPut updates the TMDB key.
func (s *Server) movieTmdbPut(c *gin.Context) {
	if !isAdmin(c) {
		apiError(c, http.StatusForbidden, "forbidden", "需要管理员权限")
		return
	}
	var req struct {
		APIKey   string `json:"api_key"`
		Language string `json:"language"`
	}
	if !bindJSON(c, &req) {
		return
	}
	settings := s.App.LoadSettings()
	if req.APIKey != "" && req.APIKey != keepSecret {
		settings.TMDB.APIKey = strings.TrimSpace(req.APIKey)
	}
	if req.Language != "" {
		settings.TMDB.Language = req.Language
	}
	if err := s.Store.SetSetting("tmdb", settings.TMDB); err != nil {
		apiError(c, http.StatusInternalServerError, "save_failed", "保存失败")
		return
	}
	s.audit(c, "system", "tmdb_settings", "tmdb", "更新 TMDB 配置", "ok")
	ok(c, gin.H{"configured": settings.TMDB.APIKey != "", "language": settings.TMDB.Language})
}

// getMovie returns movie details.
func (s *Server) getMovie(c *gin.Context) {
	m, err := s.Store.GetMovie(int64Param(c, "id"), currentUser(c).ID)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "影片不存在")
		return
	}
	s.Movies.Decorate(m)
	subs, _ := s.Movies.Subtitles(currentUser(c), m.ID)
	ok(c, gin.H{"movie": m, "subtitles": subs})
}

// updateMovie applies manual metadata edits.
func (s *Server) updateMovie(c *gin.Context) {
	u := currentUser(c)
	m, err := s.Store.GetMovie(int64Param(c, "id"), u.ID)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "影片不存在")
		return
	}
	var req struct {
		Title    *string `json:"title"`
		Year     *int    `json:"year"`
		Kind     *string `json:"kind"`
		Overview *string `json:"overview"`
		Genres   *string `json:"genres"`
		Poster   *string `json:"poster"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if req.Title != nil && strings.TrimSpace(*req.Title) != "" {
		m.Title = strings.TrimSpace(*req.Title)
	}
	if req.Year != nil && *req.Year >= 0 && *req.Year <= 2200 {
		m.Year = *req.Year
	}
	if req.Kind != nil {
		switch *req.Kind {
		case "movie", "series", "anime", "documentary":
			m.Kind = *req.Kind
		}
	}
	if req.Overview != nil {
		m.Overview = *req.Overview
	}
	if req.Genres != nil {
		m.Genres = *req.Genres
	}
	if req.Poster != nil {
		// Only local files may be referenced; remote URLs are not fetched.
		if strings.HasPrefix(*req.Poster, "/") {
			m.Poster = *req.Poster
		}
	}
	m.Scraped = true
	if err := s.Store.UpdateMovieMeta(m); err != nil {
		apiError(c, http.StatusInternalServerError, "update_failed", "保存失败")
		return
	}
	s.audit(c, "movie", "update", m.Title, "手动编辑影片信息", "ok")
	ok(c, m)
}

// deleteMovie removes a library entry, optionally deleting the file.
func (s *Server) deleteMovie(c *gin.Context) {
	u := currentUser(c)
	m, err := s.Store.GetMovie(int64Param(c, "id"), u.ID)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "影片不存在")
		return
	}
	if c.Query("delete_file") == "1" {
		root := s.Movies.Root(u)
		if m.Path != "" && strings.HasPrefix(m.Path, root+string(os.PathSeparator)) {
			_ = os.Remove(m.Path)
		}
	}
	if err := s.Store.DeleteMovieRecord(m.ID); err != nil {
		apiError(c, http.StatusInternalServerError, "delete_failed", "删除失败")
		return
	}
	s.Files.Invalidate(u.ID)
	s.audit(c, "movie", "delete", m.Title, "移除影片", "ok")
	ok(c, gin.H{"message": "已移除"})
}

// movieScan starts a library scan job.
func (s *Server) movieScan(c *gin.Context) {
	u := currentUser(c)
	job := s.App.Jobs.Start("扫描影视库", func(ctx context.Context, report func(int, string)) error {
		added, err := s.Movies.Scan(ctx, u, report)
		if err == nil {
			report(100, fmt.Sprintf("扫描完成，新增 %d 部影片", added))
		}
		s.Files.Invalidate(u.ID)
		return err
	})
	created(c, job)
}

// movieScrape starts a metadata scraping job.
func (s *Server) movieScrape(c *gin.Context) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	_ = c.ShouldBindJSON(&req)
	u := currentUser(c)
	settings := s.App.LoadSettings()
	if settings.TMDB.APIKey == "" {
		apiError(c, http.StatusBadRequest, "tmdb_not_configured", "请先在影视中心配置 TMDB API Key")
		return
	}
	tmdb := media.NewTMDB(settings.TMDB.APIKey, settings.TMDB.Language)
	job := s.App.Jobs.Start("刮削影视元数据", func(ctx context.Context, report func(int, string)) error {
		okCount, failed, err := s.Movies.ScrapeAll(ctx, u, req.IDs, tmdb, report)
		if err == nil {
			report(100, fmt.Sprintf("刮削完成：成功 %d，失败 %d", okCount, failed))
			if failed > 0 {
				return fmt.Errorf("%d 部影片刮削失败", failed)
			}
		}
		return err
	})
	created(c, job)
}

// movieStream streams a movie with Range support.
func (s *Server) movieStream(c *gin.Context) {
	path, m, err := s.Movies.StreamPath(currentUser(c), int64Param(c, "id"))
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", err.Error())
		return
	}
	f, err := os.Open(path)
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "文件无法打开")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		apiError(c, http.StatusInternalServerError, "stat_failed", "读取失败")
		return
	}
	ctype := mime.TypeByExtension(filepath.Ext(path))
	if ctype == "" || strings.Contains(ctype, "mkv") {
		ctype = "video/mp4"
	}
	c.Header("Content-Type", ctype)
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "private, max-age=3600")
	http.ServeContent(c.Writer, c.Request, m.FileName, info.ModTime(), f)
}

// moviePoster serves a movie poster from the local cache.
func (s *Server) moviePoster(c *gin.Context) {
	data, err := s.Movies.Poster(int64Param(c, "id"), currentUser(c).ID, intParam(c, "size", 400))
	if err != nil {
		apiError(c, http.StatusNotFound, "no_poster", "没有海报")
		return
	}
	c.Header("Cache-Control", "private, max-age=86400")
	c.Data(http.StatusOK, "image/jpeg", data)
}

// movieSubtitles lists sidecar subtitles.
func (s *Server) movieSubtitles(c *gin.Context) {
	subs, err := s.Movies.Subtitles(currentUser(c), int64Param(c, "id"))
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", err.Error())
		return
	}
	ok(c, subs)
}

// movieSubtitleFile returns a subtitle converted to WebVTT.
func (s *Server) movieSubtitleFile(c *gin.Context) {
	data, ctype, err := s.Movies.SubtitleVTT(currentUser(c), int64Param(c, "id"), c.Param("name"))
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", err.Error())
		return
	}
	c.Header("Cache-Control", "private, max-age=3600")
	c.Data(http.StatusOK, ctype, data)
}

// maskSecret renders a configured secret for display.
func maskSecret(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) <= 8 {
		return "••••••••"
	}
	return secret[:4] + "••••" + secret[len(secret)-4:]
}
