package api

import (
	"bytes"
	"context"
	"encoding/csv"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"privacyos/backend/internal/store"
)

// logsAudit returns filtered audit records.
func (s *Server) logsAudit(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	page, size := pageParams(c, 50)
	q := store.AuditQuery{
		Category: c.Query("category"), Username: c.Query("user"), Action: c.Query("action"),
		From: c.Query("from"), To: c.Query("to"), Page: page, PageSize: size,
	}
	items, total, err := s.Store.QueryAudit(q)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取审计日志失败")
		return
	}
	paginated(c, items, total, page, size)
}

// logsAuditExport streams audit records as CSV.
func (s *Server) logsAuditExport(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	items, _, err := s.Store.QueryAudit(store.AuditQuery{
		Category: c.Query("category"), Username: c.Query("user"), Action: c.Query("action"),
		From: c.Query("from"), To: c.Query("to"), Page: 1, PageSize: 10000,
	})
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取审计日志失败")
		return
	}
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM for spreadsheet apps
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"时间", "用户", "类别", "操作", "对象", "详情", "IP", "结果"})
	for _, e := range items {
		_ = w.Write([]string{e.CreatedAt, e.Username, e.Category, e.Action, e.Target, e.Detail, e.IP, e.Result})
	}
	w.Flush()
	name := "privacyos-audit-" + time.Now().Format("20060102-150405") + ".csv"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
	s.audit(c, "system", "audit_export", name, "导出审计日志", "ok")
}

// logsLogin returns recent login attempts.
func (s *Server) logsLogin(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	attempts, err := s.Store.RecentLoginAttempts(intParam(c, "limit", 200))
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取登录日志失败")
		return
	}
	ok(c, attempts)
}

// logsSystem returns system events and recent journal lines.
func (s *Server) logsSystem(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	events, _ := s.Store.RecentAudit(150, "system", "destroy", "network", "storage", "docker", "app")
	ok(c, gin.H{"events": events, "journal": s.journalTail(c.Request.Context(), 150)})
}

// journalTail reads recent systemd journal lines (read-only).
func (s *Server) journalTail(ctx context.Context, lines int) []string {
	if s.Cfg.Dev {
		return []string{
			"2026-05-12T09:12:01+0800 privacyos systemd[1]: Started PrivacyOS Web Panel.",
			"2026-05-12T09:12:02+0800 privacyos privacyos-server[1234]: listening on :8080",
			"2026-05-12T09:12:05+0800 privacyos systemd[1]: Started Samba SMB Daemon.",
			"2026-05-12T09:20:11+0800 privacyos smbd[1543]: session opened for user demo",
			"2026-05-12T10:02:44+0800 privacyos systemd[1]: docker.service: Deactivated successfully.",
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "journalctl", "-n", strconv.Itoa(lines), "--no-pager", "-o", "short-iso")
	out, err := cmd.Output()
	if err != nil {
		return []string{}
	}
	rows := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(rows) == 1 && rows[0] == "" {
		return []string{}
	}
	return rows
}

// logsDestroy returns destruction events (Pro only).
func (s *Server) logsDestroy(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	if !s.Pro() {
		ok(c, []store.DestroyEvent{})
		return
	}
	events, err := s.Store.ListDestroyEvents(intParam(c, "limit", 100))
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取销毁日志失败")
		return
	}
	ok(c, events)
}
