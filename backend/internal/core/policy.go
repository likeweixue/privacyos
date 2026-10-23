package core

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"privacyos/backend/internal/store"
)

// LoginGate describes whether a login attempt may proceed right now.
type LoginGate struct {
	Allowed          bool `json:"allowed"`
	Locked           bool `json:"locked"`
	RetryAfter       int  `json:"retry_after"`
	Failures         int  `json:"failures"`
	MaxAttempts      int  `json:"max_attempts"`
	DestroyThreshold int  `json:"destroy_threshold"`
	DestroyEnabled   bool `json:"destroy_enabled"`
}

// LoginOutcome is the result of recording a failed login.
type LoginOutcome struct {
	Failures         int  `json:"failures"`
	Locked           bool `json:"locked"`
	RetryAfter       int  `json:"retry_after"`
	DestroyTriggered bool `json:"destroy_triggered"`
	Message          string `json:"message"`
}

// BackoffDelay computes the exponential delay before the next attempt.
// The first *delayFrom-1 failures are free, then base, 2*base, 4*base ...
func BackoffDelay(failures, delayFrom, base, max int) time.Duration {
	if failures < delayFrom || base <= 0 {
		return 0
	}
	exp := failures - delayFrom
	if exp > 20 {
		exp = 20
	}
	d := time.Duration(base) * time.Second * time.Duration(1<<uint(exp))
	if max > 0 && d > time.Duration(max)*time.Second {
		d = time.Duration(max) * time.Second
	}
	return d
}

// LoginGateFor evaluates the lockout/delay state for a username.
func (a *App) LoginGateFor(username string) LoginGate {
	s := a.LoadSettings()
	gate := LoginGate{
		Allowed: true, MaxAttempts: s.Security.MaxAttempts,
		DestroyThreshold: s.Destroy.Threshold, DestroyEnabled: a.Pro() && s.Destroy.Enabled,
	}
	now := time.Now()
	if u, err := a.Store.GetUserByUsername(username); err == nil {
		if u.LockedUntil != "" && store.Time(u.LockedUntil).After(now) {
			gate.Allowed = false
			gate.Locked = true
			gate.RetryAfter = int(time.Until(store.Time(u.LockedUntil)).Seconds()) + 1
			return gate
		}
	}
	if !s.Security.LockoutEnabled {
		return gate
	}
	windowStart := now.Add(-time.Duration(s.Security.WindowSeconds) * time.Second).UTC().Format(time.RFC3339)
	failures, err := a.Store.CountFailedAttempts(username, windowStart)
	if err != nil {
		return gate
	}
	gate.Failures = failures
	if s.Security.DelayEnabled && failures >= s.Security.DelayFrom {
		last, err := a.Store.LastFailedAttempt(username)
		if err == nil && last != "" {
			next := store.Time(last).Add(BackoffDelay(failures, s.Security.DelayFrom,
				s.Security.BaseDelaySeconds, s.Security.MaxDelaySeconds))
			if next.After(now) {
				gate.Allowed = false
				gate.RetryAfter = int(time.Until(next).Seconds()) + 1
			}
		}
	}
	return gate
}

// RecordLoginFailure logs a failure and applies lock/destroy policy.
func (a *App) RecordLoginFailure(u *store.User, ip string) LoginOutcome {
	s := a.LoadSettings()
	_ = a.Store.RecordAttempt(u.Username, ip, false, "bad_password")
	windowStart := time.Now().Add(-time.Duration(s.Security.WindowSeconds) * time.Second).UTC().Format(time.RFC3339)
	failures, _ := a.Store.CountFailedAttempts(u.Username, windowStart)

	out := LoginOutcome{Failures: failures}
	if a.Pro() && s.Destroy.Enabled && failures >= s.Destroy.Threshold && a.canAutoDestroy(u) {
		a.TriggerDestroy(u.Username, fmt.Sprintf("wrong_password_count=%d", failures))
		out.DestroyTriggered = true
		out.Locked = true
		out.Message = "账号已锁定"
		_ = a.Store.SetStatus(u.ID, "disabled")
		_ = a.Store.RevokeUserSessions(u.ID)
		return out
	}
	if s.Security.LockoutEnabled && failures >= s.Security.MaxAttempts {
		until := time.Now().Add(time.Duration(s.Security.LockSeconds) * time.Second).UTC().Format(time.RFC3339)
		_ = a.Store.LockUntil(u.ID, until)
		_ = a.Store.RevokeUserSessions(u.ID)
		out.Locked = true
		out.RetryAfter = s.Security.LockSeconds
		out.Message = "密码错误次数过多，账号已临时锁定"
		return out
	}
	if s.Security.DelayEnabled {
		out.RetryAfter = int(BackoffDelay(failures, s.Security.DelayFrom,
			s.Security.BaseDelaySeconds, s.Security.MaxDelaySeconds).Seconds())
	}
	return out
}

// canAutoDestroy prevents an automatic policy from bricking the last
// administrator account. Manual destruction is still available to an admin.
func (a *App) canAutoDestroy(u *store.User) bool {
	if u.Role != "admin" {
		return true
	}
	admins, err := a.Store.CountUsers("admin")
	if err != nil {
		return false
	}
	if admins <= 1 {
		_ = a.Store.Audit(&store.AuditLog{
			Username: "system", Category: "destroy", Action: "destroy_skipped",
			Target: u.Username, Detail: "拒绝自动销毁唯一管理员账号", Result: "blocked",
		})
		return false
	}
	return true
}

// TriggerDestroy starts an asynchronous destruction job.
func (a *App) TriggerDestroy(username, trigger string) *Job {
	s := a.LoadSettings()
	return a.Jobs.Start("destroy:"+username, func(ctx context.Context, report func(int, string)) error {
		return a.ExecuteDestroy(ctx, username, s.Destroy, trigger, report)
	})
}

// ExecuteDestroy performs the configured destruction method for one user.
func (a *App) ExecuteDestroy(ctx context.Context, username string, d DestroySettings,
	trigger string, report func(int, string)) error {

	u, err := a.Store.GetUserByUsername(username)
	if err != nil {
		return fmt.Errorf("用户不存在: %w", err)
	}
	event := &store.DestroyEvent{Username: username, Trigger: trigger, Method: d.Method, Status: "running"}
	_ = a.Store.AddDestroyEvent(event)
	report(2, "开始销毁流程")

	method := d.Method
	detail := ""
	switch method {
	case "key_destroy":
		if a.VaultExists(username) {
			detail, err = a.DestroyVault(ctx, u)
		} else {
			switch d.Fallback {
			case "overwrite":
				method = "overwrite"
			case "delete":
				method = "delete"
			default:
				err = errors.New("未找到该用户的加密卷，且未配置回退销毁方式")
			}
		}
	case "overwrite":
	case "delete":
	default:
		err = fmt.Errorf("未知销毁方式 %q", method)
	}

	if err == nil {
		// The encrypted home is irrecoverable after a key destroy; unencrypted
		// library directories are overwritten as well so that no plaintext
		// remains on the data disk.
		var files, bytes int64
		files, bytes, err = a.wipeUserData(ctx, u, method == "overwrite" || method == "key_destroy", report)
		if err == nil {
			detail += fmt.Sprintf("；已清理 %d 个文件（%s），销毁方式：%s", files, HumanBytes(bytes), method)
		}
	}
	if err != nil {
		event.Status = "failed"
		event.Detail = err.Error()
		event.Method = method
		_ = a.Store.UpdateDestroyEvent(event.ID, "failed", method, err.Error())
		_ = a.Store.Audit(&store.AuditLog{
			Username: "system", Category: "destroy", Action: "destroy_failed",
			Target: username, Detail: err.Error(), Result: "failed",
		})
		return err
	}

	_ = a.Store.RevokeUserSessions(u.ID)
	_ = a.Store.SetStatus(u.ID, "disabled")
	_ = a.Store.ClearLoginAttempts(username)
	_ = a.Store.Audit(&store.AuditLog{
		Username: "system", Category: "destroy", Action: "destroy_completed",
		Target: username, Detail: fmt.Sprintf("trigger=%s method=%s %s", trigger, method, detail), Result: "ok",
	})
	event.Status = "completed"
	event.Method = method
	event.Detail = detail
	_ = a.Store.UpdateDestroyEvent(event.ID, "completed", method, detail)

	if !d.Silent || d.NotifyAdmin {
		_ = a.Store.Notify("critical", "数据销毁已执行", fmt.Sprintf("用户 %s 触发销毁（%s），方式：%s", username, trigger, method))
		a.Notify.Send(context.Background(), "PrivacyOS 数据销毁", fmt.Sprintf("用户 %s 的数据已销毁，方式 %s。", username, method))
	}
	report(100, "销毁完成")
	return nil
}

// wipeUserData erases all data directories of a user, optionally overwriting.
func (a *App) wipeUserData(ctx context.Context, u *store.User, overwrite bool,
	report func(int, string)) (int64, int64, error) {

	roots := []string{
		filepath.Join(a.Cfg.DataDir, "users", u.Username),
		filepath.Join(a.Cfg.DataDir, "photos", u.Username),
		filepath.Join(a.Cfg.DataDir, "movies", u.Username),
	}
	items, _ := a.Store.ListTrash(u.ID)
	for _, it := range items {
		roots = append(roots, it.TrashPath)
	}

	// Count files first to report progress.
	var total int64
	for _, root := range roots {
		_ = filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
			if err == nil && info != nil && info.Mode().IsRegular() {
				total++
			}
			return nil
		})
	}
	report(10, fmt.Sprintf("准备销毁 %d 个文件", total))

	var done, bytes int64
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			if overwrite {
				if err := overwriteFile(path, info.Size()); err != nil {
					return err
				}
			}
			done++
			bytes += info.Size()
			if total > 0 && done%16 == 0 {
				report(10+int(done*80/total), "正在销毁")
			}
			return nil
		})
		if err != nil {
			return done, bytes, err
		}
		if err := os.RemoveAll(root); err != nil {
			return done, bytes, err
		}
	}
	for _, it := range items {
		_ = a.Store.DeleteTrashItem(it.ID)
	}
	// Shares belonging to the user are revoked as well.
	if shares, err := a.Store.ListShares(u.ID, false); err == nil {
		for _, sh := range shares {
			_ = a.Store.DeleteShare(sh.ID)
		}
	}
	report(95, "清理索引")
	_, _ = a.Store.DB().Exec(`DELETE FROM photos WHERE owner_id=?`, u.ID)
	_, _ = a.Store.DB().Exec(`DELETE FROM movies WHERE owner_id=?`, u.ID)
	return done, bytes, nil
}

// overwriteFile replaces a file's content with random data and fsyncs it.
func overwriteFile(path string, size int64) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return nil // already gone
	}
	defer f.Close()
	buf := make([]byte, 64*1024)
	written := int64(0)
	for written < size {
		n := int64(len(buf))
		if size-written < n {
			n = size - written
		}
		if _, err := rand.Read(buf[:n]); err != nil {
			return err
		}
		if _, err := f.Write(buf[:n]); err != nil {
			return err
		}
		written += n
	}
	_ = f.Sync()
	return nil
}

