package core

import (
	"os"
	"path/filepath"
	"time"

	"privacyos/backend/internal/store"
)

// RunJanitor performs periodic retention work: expired sessions, trash items
// and accounts whose cooling period has elapsed.
func (a *App) RunJanitor() {
	_ = a.Store.PurgeExpiredSessions()
	_ = a.Store.PurgeBefore(time.Now().AddDate(0, 0, -90).UTC().Format(time.RFC3339))

	now := store.Now()
	if items, err := a.Store.ExpiredTrash(now); err == nil {
		for _, it := range items {
			if it.TrashPath != "" {
				_ = os.RemoveAll(it.TrashPath)
			}
			_ = a.Store.DeleteTrashItem(it.ID)
		}
	}

	users, err := a.Store.ListDeletedUsers()
	if err != nil {
		return
	}
	for _, u := range users {
		if u.DeleteAfter == "" || u.DeleteAfter > now {
			continue
		}
		a.purgeUserAccount(&u)
	}
}

// purgeUserAccount permanently removes a user's account and data.
func (a *App) purgeUserAccount(u *store.User) {
	for _, dir := range []string{
		filepath.Join(a.Cfg.DataDir, "users", u.Username),
		filepath.Join(a.Cfg.DataDir, "photos", u.Username),
		filepath.Join(a.Cfg.DataDir, "movies", u.Username),
		filepath.Join(a.Cfg.DataDir, "trash", u.Username),
	} {
		_ = os.RemoveAll(dir)
	}
	if items, err := a.Store.ListTrash(u.ID); err == nil {
		for _, it := range items {
			_ = os.RemoveAll(it.TrashPath)
		}
	}
	if err := a.Store.DeleteUser(u.ID); err != nil {
		return
	}
	_ = a.Store.Audit(&store.AuditLog{
		Username: "system", Category: "user", Action: "purge_account",
		Target: u.Username, Detail: "冷静期结束，账号与数据已永久删除", Result: "ok",
	})
}
