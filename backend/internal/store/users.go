package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("not found")

const userColumns = `id, username, display_name, password_hash, role, status,
	quota_bytes, permissions, preferences, totp_secret, totp_enabled,
	COALESCE(locked_until, ''), created_at, updated_at,
	COALESCE(last_login_at, ''), last_login_ip, COALESCE(delete_after, '')`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(sc rowScanner) (*User, error) {
	var u User
	var perms, prefs string
	err := sc.Scan(&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &u.Role, &u.Status,
		&u.QuotaBytes, &perms, &prefs, &u.TOTPSecret, &u.TOTPEnabled,
		&u.LockedUntil, &u.CreatedAt, &u.UpdatedAt,
		&u.LastLoginAt, &u.LastLoginIP, &u.DeleteAfter)
	if err != nil {
		return nil, err
	}
	u.Permissions = DefaultPermissions(u.Role)
	_ = json.Unmarshal([]byte(perms), &u.Permissions)
	_ = json.Unmarshal([]byte(prefs), &u.Preferences)
	if u.Preferences.Lang == "" {
		u.Preferences.Lang = "zh-CN"
	}
	if u.Preferences.Theme == "" {
		u.Preferences.Theme = "dark"
	}
	return &u, nil
}

// CreateUser inserts a user and fills in ID/created timestamps.
func (s *Store) CreateUser(u *User) error {
	now := Now()
	if u.CreatedAt == "" {
		u.CreatedAt = now
	}
	u.UpdatedAt = now
	if u.Status == "" {
		u.Status = "active"
	}
	if u.Role == "" {
		u.Role = "user"
	}
	if u.Permissions == (Permissions{}) {
		u.Permissions = DefaultPermissions(u.Role)
	}
	res, err := s.db.Exec(`INSERT INTO users
		(username, display_name, password_hash, role, status, quota_bytes, permissions,
		 preferences, totp_secret, totp_enabled, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		u.Username, u.DisplayName, u.PasswordHash, u.Role, u.Status, u.QuotaBytes,
		JSONMap(u.Permissions), JSONMap(u.Preferences), u.TOTPSecret, u.TOTPEnabled,
		u.CreatedAt, u.UpdatedAt)
	if err != nil {
		return err
	}
	u.ID, _ = res.LastInsertId()
	return nil
}

// GetUserByUsername looks up an account by login name.
func (s *Store) GetUserByUsername(username string) (*User, error) {
	row := s.db.QueryRow(`SELECT `+userColumns+` FROM users WHERE username = ?`, username)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// GetUserByID looks up an account by ID.
func (s *Store) GetUserByID(id int64) (*User, error) {
	row := s.db.QueryRow(`SELECT `+userColumns+` FROM users WHERE id = ?`, id)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// ListUsers returns accounts ordered by creation date.
func (s *Store) ListUsers(query string, includeDeleted bool) ([]User, error) {
	q := `SELECT ` + userColumns + ` FROM users`
	var where []string
	var args []any
	if query != "" {
		where = append(where, `(username LIKE ? ESCAPE '\' OR display_name LIKE ? ESCAPE '\')`)
		args = append(args, like(query), like(query))
	}
	if !includeDeleted {
		where = append(where, `status != 'pending_deletion'`)
	}
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY id ASC"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

// ListDeletedUsers returns accounts awaiting the end of the cooling period.
func (s *Store) ListDeletedUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT `+userColumns+` FROM users WHERE status = 'pending_deletion' ORDER BY delete_after ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

// UpdateUser persists mutable profile fields.
func (s *Store) UpdateUser(u *User) error {
	u.UpdatedAt = Now()
	_, err := s.db.Exec(`UPDATE users SET display_name=?, role=?, status=?, quota_bytes=?,
		permissions=?, preferences=?, totp_secret=?, totp_enabled=?, locked_until=?,
		delete_after=?, updated_at=? WHERE id=?`,
		u.DisplayName, u.Role, u.Status, u.QuotaBytes, JSONMap(u.Permissions),
		JSONMap(u.Preferences), u.TOTPSecret, u.TOTPEnabled, nullIfEmpty(u.LockedUntil),
		nullIfEmpty(u.DeleteAfter), u.UpdatedAt, u.ID)
	return err
}

// SetPassword updates the password hash and revokes sessions.
func (s *Store) SetPassword(id int64, hash string) error {
	if _, err := s.db.Exec(`UPDATE users SET password_hash=?, updated_at=? WHERE id=?`, hash, Now(), id); err != nil {
		return err
	}
	return s.RevokeUserSessions(id)
}

// UpdatePassword changes the password hash without revoking sessions.
func (s *Store) UpdatePassword(id int64, hash string) error {
	_, err := s.db.Exec(`UPDATE users SET password_hash=?, updated_at=? WHERE id=?`, hash, Now(), id)
	return err
}

// SetStatus changes account status.
func (s *Store) SetStatus(id int64, status string) error {
	_, err := s.db.Exec(`UPDATE users SET status=?, updated_at=? WHERE id=?`, status, Now(), id)
	return err
}

// MarkPendingDeletion flags an account for deletion after the cooling period.
func (s *Store) MarkPendingDeletion(id int64, deleteAfter string) error {
	now := Now()
	_, err := s.db.Exec(`UPDATE users SET status='pending_deletion', delete_after=?, locked_until=?, updated_at=? WHERE id=?`,
		deleteAfter, now, now, id)
	return err
}

// RestoreUser returns a pending-deletion account to active state.
func (s *Store) RestoreUser(id int64) error {
	_, err := s.db.Exec(`UPDATE users SET status='active', delete_after=NULL, locked_until=NULL, updated_at=? WHERE id=?`, Now(), id)
	return err
}

// DeleteUser permanently removes an account (cascades sessions/shares/etc.).
func (s *Store) DeleteUser(id int64) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id=?`, id)
	return err
}

// TouchLogin records a successful login.
func (s *Store) TouchLogin(id int64, ip string) error {
	_, err := s.db.Exec(`UPDATE users SET last_login_at=?, last_login_ip=?, locked_until=NULL WHERE id=?`, Now(), ip, id)
	return err
}

// LockUntil sets the temporary lock expiry timestamp.
func (s *Store) LockUntil(id int64, until string) error {
	_, err := s.db.Exec(`UPDATE users SET locked_until=?, updated_at=? WHERE id=?`, until, Now(), id)
	return err
}

// Unlock clears the lock timestamp.
func (s *Store) Unlock(id int64) error {
	_, err := s.db.Exec(`UPDATE users SET locked_until=NULL, updated_at=? WHERE id=?`, Now(), id)
	return err
}

// CountUsers returns the number of accounts with the given role ("" = all).
func (s *Store) CountUsers(role string) (int, error) {
	q := `SELECT COUNT(*) FROM users WHERE status != 'pending_deletion'`
	args := []any{}
	if role != "" {
		q += ` AND role = ?`
		args = append(args, role)
	}
	var n int
	err := s.db.QueryRow(q, args...).Scan(&n)
	return n, err
}

// ---------- sessions ----------

// CreateSession stores a new login session.
func (s *Store) CreateSession(sess *Session) error {
	_, err := s.db.Exec(`INSERT INTO sessions (id, user_id, ip, user_agent, created_at, last_seen, expires_at)
		VALUES (?,?,?,?,?,?,?)`, sess.ID, sess.UserID, sess.IP, sess.UserAgent, sess.CreatedAt, sess.LastSeen, sess.ExpiresAt)
	return err
}

// GetSession returns a session if it exists and is not revoked.
func (s *Store) GetSession(id string) (*Session, error) {
	var sess Session
	err := s.db.QueryRow(`SELECT id, user_id, ip, user_agent, created_at, last_seen, expires_at,
		COALESCE(revoked_at, '') FROM sessions WHERE id=?`, id).
		Scan(&sess.ID, &sess.UserID, &sess.IP, &sess.UserAgent, &sess.CreatedAt, &sess.LastSeen, &sess.ExpiresAt, &sess.RevokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

// TouchSession updates the last-seen time.
func (s *Store) TouchSession(id string) {
	_, _ = s.db.Exec(`UPDATE sessions SET last_seen=? WHERE id=?`, Now(), id)
}

// RevokeSession revokes a single session.
func (s *Store) RevokeSession(id string) error {
	_, err := s.db.Exec(`UPDATE sessions SET revoked_at=? WHERE id=?`, Now(), id)
	return err
}

// RevokeUserSessions revokes every session of a user except optionally one.
func (s *Store) RevokeUserSessions(userID int64) error {
	_, err := s.db.Exec(`UPDATE sessions SET revoked_at=? WHERE user_id=? AND revoked_at IS NULL`, Now(), userID)
	return err
}

// ListUserSessions lists active sessions for a user.
func (s *Store) ListUserSessions(userID int64) ([]Session, error) {
	rows, err := s.db.Query(`SELECT id, user_id, ip, user_agent, created_at, last_seen, expires_at,
		COALESCE(revoked_at, '') FROM sessions WHERE user_id=? AND revoked_at IS NULL ORDER BY last_seen DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.UserID, &sess.IP, &sess.UserAgent, &sess.CreatedAt,
			&sess.LastSeen, &sess.ExpiresAt, &sess.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

// PurgeExpiredSessions deletes sessions past their expiry.
func (s *Store) PurgeExpiredSessions() error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, Now())
	return err
}

// ---------- login attempts ----------

// RecordAttempt logs an authentication attempt.
func (s *Store) RecordAttempt(username, ip string, success bool, reason string) error {
	_, err := s.db.Exec(`INSERT INTO login_attempts (username, ip, success, reason, created_at)
		VALUES (?,?,?,?,?)`, username, ip, success, reason, Now())
	return err
}

// CountFailedAttempts counts failures for a username since the given timestamp.
func (s *Store) CountFailedAttempts(username, since string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM login_attempts
		WHERE username=? AND success=0 AND created_at >= ?`, username, since).Scan(&n)
	return n, err
}

// LastFailedAttempt returns the timestamp of the most recent failure.
func (s *Store) LastFailedAttempt(username string) (string, error) {
	var ts string
	err := s.db.QueryRow(`SELECT COALESCE(MAX(created_at), '') FROM login_attempts
		WHERE username=? AND success=0`, username).Scan(&ts)
	return ts, err
}

// RecentLoginAttempts returns the newest attempt records.
func (s *Store) RecentLoginAttempts(limit int) ([]LoginAttempt, error) {
	rows, err := s.db.Query(`SELECT id, username, ip, success, reason, created_at
		FROM login_attempts ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LoginAttempt
	for rows.Next() {
		var a LoginAttempt
		if err := rows.Scan(&a.ID, &a.Username, &a.IP, &a.Success, &a.Reason, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ClearLoginAttempts removes failure history for a user (after unlock/reset).
func (s *Store) ClearLoginAttempts(username string) error {
	_, err := s.db.Exec(`DELETE FROM login_attempts WHERE username=?`, username)
	return err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
