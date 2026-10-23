package api

import (
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/webdav"

	"privacyos/backend/internal/auth"
	"privacyos/backend/internal/config"
	"privacyos/backend/internal/core"
	"privacyos/backend/internal/dockerx"
	"privacyos/backend/internal/files"
	"privacyos/backend/internal/media"
	"privacyos/backend/internal/monitor"
	"privacyos/backend/internal/shares"
	"privacyos/backend/internal/storage"
	"privacyos/backend/internal/store"
	"privacyos/backend/internal/system"
)

const (
	ctxUser    = "privacyos.user"
	ctxSession = "privacyos.session"
	ctxClaims  = "privacyos.claims"
)

// Server bundles the HTTP API and its dependencies.
type Server struct {
	Cfg     *config.Config
	App     *core.App
	Store   *store.Store
	Tokens  *auth.Manager
	Files   *files.Service
	Storage *storage.Service
	Shares  *shares.Service
	Photos  *media.Photos
	Movies  *media.Movies
	Docker  *dockerx.Service
	Monitor *monitor.Service
	System  *system.Service

	router   *gin.Engine
	attempts *rateLimiter
}

// New builds the API server.
func New(cfg *config.Config, app *core.App) *Server {
	if cfg.Dev {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	s := &Server{
		Cfg:      cfg,
		App:      app,
		Store:    app.Store,
		Tokens:   auth.New(cfg.Secret),
		Files:    files.New(cfg, app.Store),
		Storage:  storage.New(cfg, app.Store),
		Shares:   shares.New(cfg, app.Store),
		Photos:   media.NewPhotos(cfg, app.Store),
		Movies:   media.NewMovies(cfg, app.Store),
		Docker:   dockerx.New(cfg),
		Monitor:  monitor.New(cfg),
		System:   system.New(cfg, app.Store),
		attempts: newRateLimiter(20, time.Minute),
	}
	s.router = s.buildRouter()
	return s
}

// Pro reports whether this server runs the Professional edition.
func (s *Server) Pro() bool { return s.Cfg.Pro() }

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler { return s.router }

// buildRouter wires all routes.
func (s *Server) buildRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), s.securityHeaders())
	if s.Cfg.Dev {
		r.Use(devCORS())
	}
	r.MaxMultipartMemory = 32 << 20

	r.GET("/healthz", s.health)

	api := r.Group("/api/v1")
	{
		api.GET("/system/public", s.publicInfo)
		api.POST("/auth/login", s.loginLimiter(), s.login)
		api.POST("/auth/refresh", s.refresh)

		authed := api.Group("")
		authed.Use(s.authRequired())
		{
			authed.GET("/auth/me", s.me)
			authed.POST("/auth/logout", s.logout)
			authed.POST("/auth/password", s.changePassword)
			authed.GET("/auth/sessions", s.sessionsList)
			authed.DELETE("/auth/sessions/:id", s.sessionRevoke)
			authed.POST("/auth/2fa/setup", s.totpSetup)
			authed.POST("/auth/2fa/enable", s.totpEnable)
			authed.POST("/auth/2fa/disable", s.totpDisable)
			authed.PATCH("/auth/preferences", s.updatePreferences)

			authed.GET("/dashboard/summary", s.dashboardSummary)
			authed.GET("/dashboard/activity", s.dashboardActivity)
			authed.GET("/notifications", s.notificationsList)
			authed.POST("/notifications/read", s.notificationsRead)
			authed.DELETE("/notifications/:id", s.notificationDelete)

			authed.GET("/jobs", s.jobsList)
			authed.POST("/jobs/:id/cancel", s.jobCancel)

			// Files
			f := authed.Group("/files")
			f.Use(s.feature("files"))
			f.GET("", s.listFiles)
			f.GET("/stat", s.statFile)
			f.POST("/mkdir", s.mkdir)
			f.POST("/rename", s.rename)
			f.POST("/move", s.move)
			f.POST("/copy", s.copy)
			f.POST("/delete", s.deleteFiles)
			f.POST("/upload", s.upload)
			f.GET("/download", s.download)
			f.GET("/archive", s.archiveDownload)
			f.POST("/extract", s.extract)
			f.GET("/search", s.search)
			f.GET("/thumb", s.fileThumb)
			f.GET("/trash", s.trashList)
			f.POST("/trash/restore", s.trashRestore)
			f.POST("/trash/empty", s.trashEmpty)
			f.GET("/usage", s.fileUsage)
			f.POST("/share", s.createShare)
			f.GET("/users", s.fileUsers)

			sh := authed.Group("/shares")
			sh.GET("", s.listShares)
			sh.GET("/:id", s.getShare)
			sh.DELETE("/:id", s.deleteShare)

			// Photos
			p := authed.Group("/photos")
			p.Use(s.feature("photos"))
			p.GET("", s.listPhotos)
			p.GET("/timeline", s.photoTimeline)
			p.GET("/map", s.photoMap)
			p.GET("/albums", s.albumsList)
			p.POST("/albums", s.albumCreate)
			p.GET("/albums/:id", s.albumGet)
			p.DELETE("/albums/:id", s.albumDelete)
			p.POST("/albums/:id/photos", s.albumAddPhotos)
			p.DELETE("/albums/:id/photos/:photoId", s.albumRemovePhoto)
			p.POST("/upload", s.photoUpload)
			p.POST("/scan", s.photoScan)
			p.GET("/:id", s.getPhoto)
			p.DELETE("/:id", s.photoDelete)
			p.GET("/:id/file", s.photoFile)
			p.GET("/:id/thumb", s.photoThumb)
			p.POST("/:id/share", s.photoShare)

			// Movies
			m := authed.Group("/movies")
			m.Use(s.feature("movies"))
			m.GET("", s.listMovies)
			m.GET("/meta", s.movieMeta)
			m.GET("/tmdb", s.movieTmdbGet)
			m.PUT("/tmdb", s.movieTmdbPut)
			m.POST("/scan", s.movieScan)
			m.POST("/scrape", s.movieScrape)
			m.GET("/:id", s.getMovie)
			m.PATCH("/:id", s.updateMovie)
			m.DELETE("/:id", s.deleteMovie)
			m.GET("/:id/stream", s.movieStream)
			m.HEAD("/:id/stream", s.movieStream)
			m.GET("/:id/poster", s.moviePoster)
			m.GET("/:id/subtitles", s.movieSubtitles)
			m.GET("/:id/subtitles/:name", s.movieSubtitleFile)

			// Docker
			d := authed.Group("/docker")
			d.Use(s.feature("docker"))
			d.GET("/status", s.dockerStatus)
			d.GET("/containers", s.dockerContainers)
			d.POST("/containers/:id/:action", s.dockerAction)
			d.DELETE("/containers/:id", s.dockerRemove)
			d.GET("/containers/:id/logs", s.dockerLogs)
			d.GET("/containers/:id/stats", s.dockerStats)
			d.GET("/images", s.dockerImages)
			d.POST("/images/pull", s.dockerPull)
			d.DELETE("/images/:id", s.dockerRemoveImage)
			d.GET("/compose", s.composeList)
			d.POST("/compose", s.composeCreate)
			d.DELETE("/compose/:name", s.composeDelete)
			d.POST("/compose/:name/up", s.composeUp)
			d.POST("/compose/:name/down", s.composeDown)
			d.GET("/compose/:name/logs", s.composeLogs)

			// Storage
			st := authed.Group("/storage")
			st.Use(s.feature("files"))
			st.GET("/disks", s.storageDisks)
			st.GET("/smart", s.storageSmart)
			st.POST("/smart/test", s.storageSmartTest)
			st.GET("/smart/tests", s.storageSmartTests)
			st.GET("/pools", s.storagePools)
			st.POST("/pools", s.storagePoolCreate)
			st.POST("/pools/:id/mount", s.storagePoolMount)
			st.POST("/pools/:id/unmount", s.storagePoolUnmount)
			st.DELETE("/pools/:id", s.storagePoolDelete)
			st.POST("/pools/:id/extend", s.storagePoolExtend)
			st.GET("/mounts", s.storageMounts)

			// Network
			n := authed.Group("/network")
			n.GET("/interfaces", s.networkInterfaces)
			n.POST("/interfaces/:name", s.networkApplyInterface)
			n.GET("/ports", s.networkPorts)
			n.GET("/shares", s.networkSharesList)
			n.POST("/shares", s.networkShareCreate)
			n.PATCH("/shares/:id", s.networkShareUpdate)
			n.DELETE("/shares/:id", s.networkShareDelete)
			n.POST("/apply", s.networkApply)
			n.GET("/ddns", s.ddnsGet)
			n.PUT("/ddns", s.ddnsPut)
			n.POST("/ddns/update", s.ddnsUpdate)
			n.GET("/dns", s.dnsGet)
			n.PUT("/dns", s.dnsPut)

			// Apps
			a := authed.Group("/apps")
			a.GET("", s.appsList)
			a.POST("/:slug/install", s.appInstall)
			a.POST("/:slug/uninstall", s.appUninstall)
			a.POST("/:slug/start", s.appStart)
			a.POST("/:slug/stop", s.appStop)
			a.GET("/:slug/status", s.appStatus)

			// Monitor
			mo := authed.Group("/monitor")
			mo.GET("/overview", s.monitorOverview)
			mo.GET("/stream", s.monitorStream)
			mo.GET("/history", s.monitorHistory)
			mo.GET("/processes", s.monitorProcesses)
			mo.POST("/processes/:pid/kill", s.monitorKill)

			// Logs
			lg := authed.Group("/logs")
			lg.GET("/audit", s.logsAudit)
			lg.GET("/audit/export", s.logsAuditExport)
			lg.GET("/login", s.logsLogin)
			lg.GET("/system", s.logsSystem)
			lg.GET("/destroy", s.logsDestroy)

			// Settings & system (admin only)
			adm := authed.Group("")
			adm.Use(s.adminOnly())
			{
				adm.GET("/users", s.listUsers)
				adm.POST("/users", s.createUser)
				adm.GET("/users/deleted", s.deletedUsers)
				adm.GET("/users/:id", s.getUser)
				adm.PATCH("/users/:id", s.updateUser)
				adm.DELETE("/users/:id", s.deleteUser)
				adm.POST("/users/:id/restore", s.restoreUser)
				adm.POST("/users/:id/password", s.resetUserPassword)
				adm.POST("/users/:id/unlock", s.unlockUser)
				adm.GET("/users/:id/usage", s.userUsage)

				adm.GET("/settings", s.getSettings)
				adm.PUT("/settings/:key", s.putSetting)

				adm.GET("/system/info", s.systemInfo)
				adm.GET("/system/updates", s.systemUpdates)
				adm.POST("/system/updates/apply", s.systemApplyUpdates)
				adm.GET("/system/backup", s.systemBackup)
				adm.POST("/system/restore", s.systemRestore)
				adm.GET("/system/ssl", s.sslInfo)
				adm.POST("/system/ssl/selfsigned", s.sslSelfSigned)
				adm.GET("/system/firewall", s.firewallGet)
				adm.PUT("/system/firewall", s.firewallPut)
				adm.POST("/system/firewall/confirm", s.firewallConfirm)
				adm.GET("/system/encryption", s.encryptionInfo)
				adm.GET("/system/vaults", s.vaultList)
				adm.POST("/system/vaults/:username/create", s.vaultCreate)
				adm.POST("/system/vaults/:username/unlock", s.vaultUnlock)
				adm.POST("/system/vaults/:username/lock", s.vaultLock)
				adm.POST("/system/destroy", s.manualDestroy)
				adm.GET("/system/destroy/policy", s.destroyPolicyGet)
				adm.PUT("/system/destroy/policy", s.destroyPolicyPut)
				adm.POST("/notifications/test", s.notificationTest)
				adm.GET("/system/audit/export", s.logsAuditExport)
			}
		}

		api.GET("/s/:token", s.publicShareInfo)
		api.POST("/s/:token/verify", s.publicShareVerify)
		api.GET("/s/:token/download", s.publicShareDownload)
	}

	// WebDAV uses non-standard HTTP verbs that Gin's Any() does not register.
	davHandler := s.webdavHandler()
	for _, method := range []string{
		http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions,
		"PROPFIND", "PROPPATCH", "MKCOL", "COPY", "MOVE", "LOCK", "UNLOCK",
	} {
		r.Handle(method, "/dav/*path", davHandler)
	}
	r.NoRoute(s.serveSPA)
	return r
}

// health returns a lightweight liveness probe.
func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "privacyos", "version": s.Cfg.Version})
}

// publicInfo returns branding for the login page.
func (s *Server) publicInfo(c *gin.Context) {
	st := s.App.LoadSettings()
	info := s.System.Info(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"name": st.System.Name, "website": st.System.Website,
		"version": s.Cfg.Version, "edition": s.Cfg.Edition, "edition_name": s.Cfg.EditionName(),
		"hostname": info.Hostname, "ips": info.IPs, "dev": s.Cfg.Dev,
		"https": st.HTTPS.Enabled,
	}})
}

// securityHeaders applies conservative browser security policies.
func (s *Server) securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		h.Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data: blob:; media-src 'self' blob:; "+
				"style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; "+
				"font-src 'self' data:; frame-src 'self'; object-src 'none'; base-uri 'self'")
		c.Next()
	}
}

// devCORS allows the Vite dev server to call the API.
func devCORS() gin.HandlerFunc {
	allowed := map[string]bool{
		"http://localhost:5173": true, "http://127.0.0.1:5173": true,
		"http://localhost:4173": true, "http://127.0.0.1:4173": true,
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowed[origin] {
			h := c.Writer.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// serveSPA serves the built frontend with history-mode fallback.
func (s *Server) serveSPA(c *gin.Context) {
	p := c.Request.URL.Path
	if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/dav") {
		apiError(c, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	webDir := s.Cfg.WebDir
	if st, err := os.Stat(webDir); err != nil || !st.IsDir() {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(placeholderHTML))
		return
	}
	clean := path.Clean("/" + p)
	target := filepath.Join(webDir, filepath.FromSlash(strings.TrimPrefix(clean, "/")))
	if !strings.HasPrefix(target, webDir) {
		target = filepath.Join(webDir, "index.html")
	}
	if st, err := os.Stat(target); err == nil && !st.IsDir() {
		if strings.HasPrefix(clean, "/assets/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		}
		c.File(target)
		return
	}
	index := filepath.Join(webDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(placeholderHTML))
		return
	}
	c.Header("Cache-Control", "no-cache")
	c.File(index)
}

// webdavHandler exposes each user's home over WebDAV with HTTP Basic auth.
func (s *Server) webdavHandler() gin.HandlerFunc {
	// A shared lock system is required so LOCK/UNLOCK work across requests.
	lockSystem := webdav.NewMemLS()
	return func(c *gin.Context) {
		username, password, ok := c.Request.BasicAuth()
		if !ok {
			c.Header("WWW-Authenticate", `Basic realm="PrivacyOS WebDAV"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		u, err := s.Store.GetUserByUsername(username)
		if err != nil || u.Status != "active" || !auth.VerifyPassword(u.PasswordHash, password) || !u.Permissions.Files {
			time.Sleep(300 * time.Millisecond)
			c.Header("WWW-Authenticate", `Basic realm="PrivacyOS WebDAV"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		handler := &webdav.Handler{
			Prefix:     "/dav",
			FileSystem: webdav.Dir(s.Files.Home(u)),
			LockSystem: lockSystem,
		}
		s.audit(c, "file", "webdav", c.Request.URL.Path, c.Request.Method, "ok")
		handler.ServeHTTP(c.Writer, c.Request)
	}
}

// audit writes an audit record without failing the request.
func (s *Server) audit(c *gin.Context, category, action, target, detail, result string) {
	u := currentUser(c)
	username := "anonymous"
	var userID int64
	if u != nil {
		username = u.Username
		userID = u.ID
	}
	if err := s.Store.Audit(&store.AuditLog{
		UserID: userID, Username: username, Category: category, Action: action,
		Target: target, Detail: detail, IP: clientIP(c), Result: result,
	}); err != nil {
		log.Printf("audit write failed: %v", err)
	}
}

const placeholderHTML = `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>PrivacyOS</title>
<style>body{background:#0b0f14;color:#e6edf3;font-family:system-ui,-apple-system,"Segoe UI",sans-serif;
display:flex;align-items:center;justify-content:center;height:100vh;margin:0}
.card{max-width:560px;padding:40px;border:1px solid #22303c;border-radius:16px;background:#121a22}
h1{margin:0 0 12px;font-size:26px;letter-spacing:.5px}code{background:#1b2733;padding:2px 6px;border-radius:6px}
p{line-height:1.7;color:#9fb0c0}a{color:#5eead4}</style></head><body><div class="card">
<h1>PrivacyOS</h1><p>后端已启动，但未找到前端构建产物。</p>
<p>开发模式请运行 <code>make dev-web</code> 或 <code>cd frontend &amp;&amp; npm run dev</code>；<br>
生产模式请先构建前端，Web 目录位于 <code>/opt/privacyos/web</code>。</p>
<p>API 健康检查：<a href="/healthz">/healthz</a></p></div></body></html>`
