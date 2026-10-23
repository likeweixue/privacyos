package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// monitorOverview returns the current metrics snapshot.
func (s *Server) monitorOverview(c *gin.Context) {
	snap := s.Monitor.Snapshot()
	info := s.System.Info(c.Request.Context())
	ok(c, gin.H{"snapshot": snap, "hostname": info.Hostname, "version": s.Cfg.Version,
		"edition": s.Cfg.Edition, "edition_name": s.Cfg.EditionName(), "uptime": info.Uptime})
}

// monitorHistory returns recent samples for chart bootstrapping.
func (s *Server) monitorHistory(c *gin.Context) {
	s.Monitor.Snapshot()
	ok(c, s.Monitor.History(intParam(c, "limit", 60)))
}

// monitorStream pushes metrics over Server-Sent Events.
func (s *Server) monitorStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, canFlush := c.Writer.(http.Flusher)
	send := func() bool {
		snap := s.Monitor.Snapshot()
		payload, err := json.Marshal(snap)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(c.Writer, "event: metrics\ndata: %s\n\n", payload); err != nil {
			return false
		}
		if canFlush {
			flusher.Flush()
		}
		return true
	}
	if !send() {
		return
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			if !send() {
				return
			}
		}
	}
}

// monitorProcesses returns the process list.
func (s *Server) monitorProcesses(c *gin.Context) {
	procs, err := s.Monitor.Processes()
	if err != nil {
		apiError(c, http.StatusInternalServerError, "process_error", err.Error())
		return
	}
	ok(c, procs)
}

// monitorKill terminates a process (admin only).
func (s *Server) monitorKill(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	pid, err := strconv.Atoi(c.Param("pid"))
	if err != nil || pid <= 1 {
		apiError(c, http.StatusBadRequest, "invalid_pid", "非法进程号")
		return
	}
	var req struct {
		Force bool `json:"force"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := s.Monitor.Kill(pid, req.Force); err != nil {
		apiError(c, http.StatusBadRequest, "kill_failed", err.Error())
		return
	}
	s.audit(c, "system", "kill_process", strconv.Itoa(pid), "结束进程", "ok")
	ok(c, gin.H{"message": "已发送终止信号"})
}

// jobsList returns background jobs.
func (s *Server) jobsList(c *gin.Context) {
	ok(c, s.App.Jobs.List(50))
}

// jobCancel aborts a background job.
func (s *Server) jobCancel(c *gin.Context) {
	if !s.App.Jobs.Cancel(c.Param("id")) {
		apiError(c, http.StatusNotFound, "not_found", "任务不存在或已结束")
		return
	}
	ok(c, gin.H{"message": "任务已取消"})
}

// dashboardSummary aggregates the dashboard payload.
func (s *Server) dashboardSummary(c *gin.Context) {
	u := currentUser(c)
	snap := s.Monitor.Snapshot()
	photos, _ := s.Store.CountPhotos(u.ID)
	movies, _ := s.Store.CountMovies(u.ID)
	shares, _ := s.Store.CountShares(u.ID)
	users, _ := s.Store.CountUsers("")
	pending, _ := s.Store.CountPendingDeletions()
	alerts, _ := s.Store.ActiveAlerts()
	unread, _ := s.Store.UnreadCount()
	recent, _ := s.Store.RecentAudit(12)
	notifications, _ := s.Store.ListNotifications(5, false)
	usage, _ := s.Files.Usage(u)
	diskAlerts := []gin.H{}
	disks, _ := s.Storage.ListDisks(c.Request.Context())
	for _, d := range disks {
		if d.Health == "failing" {
			diskAlerts = append(diskAlerts, gin.H{
				"device": d.Path, "model": d.Model, "level": "critical",
				"message": "SMART 检测异常，请立即备份并更换磁盘",
			})
		}
	}
	ok(c, gin.H{
		"snapshot": snap,
		"counts": gin.H{
			"users": users, "photos": photos, "movies": movies,
			"shares": shares, "pending_deletions": pending,
		},
		"usage": usage, "alerts": alerts, "disk_alerts": diskAlerts,
		"notifications": notifications, "unread": unread, "recent": recent,
		"edition": s.Cfg.Edition, "edition_name": s.Cfg.EditionName(), "pro": s.Cfg.Pro(),
		"version": s.Cfg.Version,
	})
}

// dashboardActivity returns recent audit entries.
func (s *Server) dashboardActivity(c *gin.Context) {
	entries, err := s.Store.RecentAudit(intParam(c, "limit", 20))
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取活动失败")
		return
	}
	ok(c, entries)
}
