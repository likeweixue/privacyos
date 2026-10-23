package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// ---------- network shares ----------

// CreateNetworkShare inserts an SMB/NFS export.
func (s *Store) CreateNetworkShare(n *NetworkShare) error {
	n.ID = NewID()
	n.CreatedAt = Now()
	p, _ := json.Marshal(n.Protocols)
	_, err := s.db.Exec(`INSERT INTO network_shares (id, name, path, comment, protocols, read_only, valid_users, guest_ok, created_at)
		VALUES (?,?,?,?,?,?,?,?,?)`, n.ID, n.Name, n.Path, n.Comment, string(p), n.ReadOnly, n.ValidUsers, n.GuestOK, n.CreatedAt)
	return err
}

// ListNetworkShares returns all configured exports.
func (s *Store) ListNetworkShares() ([]NetworkShare, error) {
	rows, err := s.db.Query(`SELECT id, name, path, comment, protocols, read_only, valid_users, guest_ok, created_at
		FROM network_shares ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NetworkShare{}
	for rows.Next() {
		var n NetworkShare
		var protocols string
		if err := rows.Scan(&n.ID, &n.Name, &n.Path, &n.Comment, &protocols, &n.ReadOnly,
			&n.ValidUsers, &n.GuestOK, &n.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(protocols), &n.Protocols)
		out = append(out, n)
	}
	return out, rows.Err()
}

// GetNetworkShare returns one export by ID.
func (s *Store) GetNetworkShare(id string) (*NetworkShare, error) {
	var n NetworkShare
	var protocols string
	err := s.db.QueryRow(`SELECT id, name, path, comment, protocols, read_only, valid_users, guest_ok, created_at
		FROM network_shares WHERE id=?`, id).
		Scan(&n.ID, &n.Name, &n.Path, &n.Comment, &protocols, &n.ReadOnly, &n.ValidUsers, &n.GuestOK, &n.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(protocols), &n.Protocols)
	return &n, nil
}

// UpdateNetworkShare persists changes to an export.
func (s *Store) UpdateNetworkShare(n *NetworkShare) error {
	p, _ := json.Marshal(n.Protocols)
	_, err := s.db.Exec(`UPDATE network_shares SET name=?, path=?, comment=?, protocols=?,
		read_only=?, valid_users=?, guest_ok=? WHERE id=?`,
		n.Name, n.Path, n.Comment, string(p), n.ReadOnly, n.ValidUsers, n.GuestOK, n.ID)
	return err
}

// DeleteNetworkShare removes an export.
func (s *Store) DeleteNetworkShare(id string) error {
	_, err := s.db.Exec(`DELETE FROM network_shares WHERE id=?`, id)
	return err
}

// ---------- storage pools ----------

// UpsertPool inserts or updates a storage pool.
func (s *Store) UpsertPool(p *StoragePool) error {
	if p.ID == "" {
		p.ID = NewID()
	}
	now := Now()
	if p.CreatedAt == "" {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	devices := JSONMap(p.Devices)
	_, err := s.db.Exec(`INSERT INTO storage_pools (id, name, level, fs, devices, mountpoint, status, size_bytes, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(name) DO UPDATE SET level=excluded.level, fs=excluded.fs, devices=excluded.devices,
			mountpoint=excluded.mountpoint, status=excluded.status, size_bytes=excluded.size_bytes, updated_at=excluded.updated_at`,
		p.ID, p.Name, p.Level, p.FS, devices, p.Mountpoint, p.Status, p.SizeBytes, p.CreatedAt, p.UpdatedAt)
	return err
}

// ListPools returns all registered pools.
func (s *Store) ListPools() ([]StoragePool, error) {
	rows, err := s.db.Query(`SELECT id, name, level, fs, devices, mountpoint, status, size_bytes, created_at, updated_at
		FROM storage_pools ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StoragePool{}
	for rows.Next() {
		var p StoragePool
		var devices string
		if err := rows.Scan(&p.ID, &p.Name, &p.Level, &p.FS, &devices, &p.Mountpoint,
			&p.Status, &p.SizeBytes, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(devices), &p.Devices)
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPool returns one pool by ID.
func (s *Store) GetPool(id string) (*StoragePool, error) {
	var p StoragePool
	var devices string
	err := s.db.QueryRow(`SELECT id, name, level, fs, devices, mountpoint, status, size_bytes, created_at, updated_at
		FROM storage_pools WHERE id=?`, id).
		Scan(&p.ID, &p.Name, &p.Level, &p.FS, &devices, &p.Mountpoint, &p.Status, &p.SizeBytes, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(devices), &p.Devices)
	return &p, nil
}

// DeletePool removes a pool record.
func (s *Store) DeletePool(id string) error {
	_, err := s.db.Exec(`DELETE FROM storage_pools WHERE id=?`, id)
	return err
}

// ---------- SMART ----------

// AddSmartTest stores a SMART self-test result.
func (s *Store) AddSmartTest(t *SmartTest) error {
	t.CreatedAt = Now()
	res, err := s.db.Exec(`INSERT INTO smart_tests (device, kind, status, summary, raw, created_at)
		VALUES (?,?,?,?,?,?)`, t.Device, t.Kind, t.Status, t.Summary, t.Raw, t.CreatedAt)
	if err != nil {
		return err
	}
	t.ID, _ = res.LastInsertId()
	return nil
}

// ListSmartTests returns the latest tests for a device (or all devices).
func (s *Store) ListSmartTests(device string, limit int) ([]SmartTest, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	q := `SELECT id, device, kind, status, summary, raw, created_at FROM smart_tests`
	args := []any{}
	if device != "" {
		q += ` WHERE device=?`
		args = append(args, device)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SmartTest{}
	for rows.Next() {
		var t SmartTest
		if err := rows.Scan(&t.ID, &t.Device, &t.Kind, &t.Status, &t.Summary, &t.Raw, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ---------- notifications & alerts ----------

// Notify stores an in-app notification.
func (s *Store) Notify(level, title, message string) error {
	_, err := s.db.Exec(`INSERT INTO notifications (level, title, message, created_at) VALUES (?,?,?,?)`,
		level, title, message, Now())
	return err
}

// ListNotifications returns recent notifications.
func (s *Store) ListNotifications(limit int, unreadOnly bool) ([]Notification, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `SELECT id, level, title, message, read, created_at FROM notifications`
	if unreadOnly {
		q += ` WHERE read=0`
	}
	q += ` ORDER BY id DESC LIMIT ?`
	rows, err := s.db.Query(q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Notification{}
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.Level, &n.Title, &n.Message, &n.Read, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// MarkNotificationsRead marks one or all notifications as read.
func (s *Store) MarkNotificationsRead(id int64) error {
	if id == 0 {
		_, err := s.db.Exec(`UPDATE notifications SET read=1 WHERE read=0`)
		return err
	}
	_, err := s.db.Exec(`UPDATE notifications SET read=1 WHERE id=?`, id)
	return err
}

// DeleteNotification removes a notification.
func (s *Store) DeleteNotification(id int64) error {
	_, err := s.db.Exec(`DELETE FROM notifications WHERE id=?`, id)
	return err
}

// UnreadCount returns the number of unread notifications.
func (s *Store) UnreadCount() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM notifications WHERE read=0`).Scan(&n)
	return n, err
}

// RaiseAlert records an alert once per key while it stays unresolved.
func (s *Store) RaiseAlert(key, level, title, message string) error {
	var id int64
	err := s.db.QueryRow(`SELECT id FROM alert_events WHERE key=? AND resolved_at IS NULL ORDER BY id DESC LIMIT 1`, key).Scan(&id)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO alert_events (key, level, title, message, created_at) VALUES (?,?,?,?,?)`,
		key, level, title, message, Now())
	return err
}

// ResolveAlert closes open alerts for a key.
func (s *Store) ResolveAlert(key string) error {
	_, err := s.db.Exec(`UPDATE alert_events SET resolved_at=? WHERE key=? AND resolved_at IS NULL`, Now(), key)
	return err
}

// ActiveAlerts returns unresolved alerts.
func (s *Store) ActiveAlerts() ([]AlertEvent, error) {
	rows, err := s.db.Query(`SELECT id, key, level, title, message, created_at, COALESCE(resolved_at, '')
		FROM alert_events WHERE resolved_at IS NULL ORDER BY id DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AlertEvent{}
	for rows.Next() {
		var a AlertEvent
		if err := rows.Scan(&a.ID, &a.Key, &a.Level, &a.Title, &a.Message, &a.CreatedAt, &a.ResolvedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ---------- apps ----------

// ListApps returns the application catalog.
func (s *Store) ListApps() ([]App, error) {
	rows, err := s.db.Query(`SELECT slug, name, category, description, icon, image, ports, compose,
		installed, status, COALESCE(installed_at, '') FROM apps ORDER BY category, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []App{}
	for rows.Next() {
		var a App
		var ports string
		if err := rows.Scan(&a.Slug, &a.Name, &a.Category, &a.Description, &a.Icon, &a.Image,
			&ports, &a.Compose, &a.Installed, &a.Status, &a.InstalledAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(ports), &a.Ports)
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetApp returns one catalog entry.
func (s *Store) GetApp(slug string) (*App, error) {
	var a App
	var ports string
	err := s.db.QueryRow(`SELECT slug, name, category, description, icon, image, ports, compose,
		installed, status, COALESCE(installed_at, '') FROM apps WHERE slug=?`, slug).
		Scan(&a.Slug, &a.Name, &a.Category, &a.Description, &a.Icon, &a.Image, &ports, &a.Compose,
			&a.Installed, &a.Status, &a.InstalledAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(ports), &a.Ports)
	return &a, nil
}

// SetAppState updates installation state for a catalog entry.
func (s *Store) SetAppState(slug string, installed bool, status string) error {
	installedAt := any(nil)
	if installed {
		installedAt = Now()
	}
	_, err := s.db.Exec(`UPDATE apps SET installed=?, status=?, installed_at=COALESCE(?, installed_at) WHERE slug=?`,
		installed, status, installedAt, slug)
	return err
}

// ---------- destroy events ----------

// AddDestroyEvent appends to the destroy audit trail.
func (s *Store) AddDestroyEvent(e *DestroyEvent) error {
	if e.CreatedAt == "" {
		e.CreatedAt = Now()
	}
	res, err := s.db.Exec(`INSERT INTO destroy_events (created_at, username, trigger, method, status, detail)
		VALUES (?,?,?,?,?,?)`, e.CreatedAt, e.Username, e.Trigger, e.Method, e.Status, e.Detail)
	if err != nil {
		return err
	}
	e.ID, _ = res.LastInsertId()
	return nil
}

// UpdateDestroyEvent completes or fails an existing destroy event.
func (s *Store) UpdateDestroyEvent(id int64, status, method, detail string) error {
	_, err := s.db.Exec(`UPDATE destroy_events SET status=?, method=?, detail=? WHERE id=?`,
		status, method, detail, id)
	return err
}

// ListDestroyEvents returns the newest destroy events.
func (s *Store) ListDestroyEvents(limit int) ([]DestroyEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id, created_at, username, trigger, method, status, detail
		FROM destroy_events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DestroyEvent{}
	for rows.Next() {
		var e DestroyEvent
		if err := rows.Scan(&e.ID, &e.CreatedAt, &e.Username, &e.Trigger, &e.Method, &e.Status, &e.Detail); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CountPendingDeletions counts accounts awaiting deletion.
func (s *Store) CountPendingDeletions() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE status='pending_deletion'`).Scan(&n)
	return n, err
}

// PurgeBefore removes audit/notification rows older than the retention window.
func (s *Store) PurgeBefore(cutoff string) error {
	if _, err := s.db.Exec(`DELETE FROM notifications WHERE created_at < ?`, cutoff); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM login_attempts WHERE created_at < ?`, cutoff); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM alert_events WHERE resolved_at IS NOT NULL AND created_at < ?`, cutoff)
	return err
}

// TodayStart returns the RFC3339 timestamp for local midnight.
func TodayStart() string {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UTC().Format(time.RFC3339)
}
