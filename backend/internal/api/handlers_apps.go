package api

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// appsList returns the application catalog.
func (s *Server) appsList(c *gin.Context) {
	apps, err := s.Store.ListApps()
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取应用列表失败")
		return
	}
	for i := range apps {
		apps[i].Compose = ""
	}
	ok(c, apps)
}

// appInstall deploys a catalog application.
func (s *Server) appInstall(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	app, err := s.Store.GetApp(c.Param("slug"))
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "应用不存在")
		return
	}
	if app.Installed {
		apiError(c, http.StatusConflict, "already_installed", "应用已安装")
		return
	}
	status := s.Docker.Status(c.Request.Context())
	if available, _ := status["available"].(bool); !available && !s.Cfg.Dev {
		apiError(c, http.StatusBadGateway, "docker_unavailable", "Docker 未运行，无法部署应用")
		return
	}
	if _, err := s.Docker.CreateCompose(app.Slug, app.Compose); err != nil {
		apiError(c, http.StatusBadRequest, "compose_error", err.Error())
		return
	}
	out, err := s.Docker.ComposeUp(c.Request.Context(), app.Slug)
	if err != nil {
		apiError(c, http.StatusBadGateway, "install_failed", "部署失败: "+err.Error())
		return
	}
	if err := s.Store.SetAppState(app.Slug, true, "running"); err != nil {
		apiError(c, http.StatusInternalServerError, "save_failed", "保存状态失败")
		return
	}
	s.audit(c, "app", "install", app.Name, "安装应用", "ok")
	ok(c, gin.H{"message": app.Name + " 已部署", "output": out})
}

// appUninstall removes an application stack.
func (s *Server) appUninstall(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	app, err := s.Store.GetApp(c.Param("slug"))
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "应用不存在")
		return
	}
	_, _ = s.Docker.ComposeDown(c.Request.Context(), app.Slug)
	_ = s.Docker.DeleteCompose(c.Request.Context(), app.Slug)
	_ = s.Store.SetAppState(app.Slug, false, "stopped")
	s.audit(c, "app", "uninstall", app.Name, "卸载应用（保留数据目录）", "ok")
	ok(c, gin.H{"message": app.Name + " 已卸载，数据目录保留在 /data/docker/compose/" + app.Slug})
}

// appStart starts an installed application.
func (s *Server) appStart(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	app, err := s.Store.GetApp(c.Param("slug"))
	if err != nil || !app.Installed {
		apiError(c, http.StatusNotFound, "not_found", "应用未安装")
		return
	}
	if _, err := s.Docker.ComposeUp(c.Request.Context(), app.Slug); err != nil {
		apiError(c, http.StatusBadGateway, "start_failed", err.Error())
		return
	}
	_ = s.Store.SetAppState(app.Slug, true, "running")
	ok(c, gin.H{"message": "已启动"})
}

// appStop stops an installed application.
func (s *Server) appStop(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	app, err := s.Store.GetApp(c.Param("slug"))
	if err != nil || !app.Installed {
		apiError(c, http.StatusNotFound, "not_found", "应用未安装")
		return
	}
	if _, err := s.Docker.ComposeDown(c.Request.Context(), app.Slug); err != nil {
		apiError(c, http.StatusBadGateway, "stop_failed", err.Error())
		return
	}
	_ = s.Store.SetAppState(app.Slug, true, "stopped")
	ok(c, gin.H{"message": "已停止"})
}

// appStatus reports live container status for an app.
func (s *Server) appStatus(c *gin.Context) {
	app, err := s.Store.GetApp(c.Param("slug"))
	if err != nil {
		apiError(c, http.StatusNotFound, "not_found", "应用不存在")
		return
	}
	containers, _ := s.Docker.ListContainers(c.Request.Context(), true)
	var running, total int
	for _, ct := range containers {
		if strings.HasPrefix(ct.Name, "privacyos-"+app.Slug) || strings.Contains(ct.Name, app.Slug) {
			total++
			if ct.State == "running" {
				running++
			}
		}
	}
	status := "stopped"
	if running > 0 {
		status = "running"
	}
	ok(c, gin.H{"installed": app.Installed, "status": status, "containers": total, "running": running,
		"ports": app.Ports, "compose_dir": filepath.Join(s.Cfg.DataDir, "docker", "compose", app.Slug)})
}
