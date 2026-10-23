package store

import (
	"database/sql"
	"errors"
)

const shareColumns = `id, token, name, path, owner_id, kind, password_hash, COALESCE(expires_at, ''),
	download_only, max_downloads, downloads, size_bytes, created_at, COALESCE(last_access_at, '')`

func scanShare(sc rowScanner) (*Share, error) {
	var sh Share
	if err := sc.Scan(&sh.ID, &sh.Token, &sh.Name, &sh.Path, &sh.OwnerID, &sh.Kind,
		&sh.PasswordHash, &sh.ExpiresAt, &sh.DownloadOnly, &sh.MaxDownloads, &sh.Downloads,
		&sh.SizeBytes, &sh.CreatedAt, &sh.LastAccessAt); err != nil {
		return nil, err
	}
	sh.HasPassword = sh.PasswordHash != ""
	return &sh, nil
}

// CreateShare inserts a share link.
func (s *Store) CreateShare(sh *Share) error {
	if sh.ID == "" {
		sh.ID = NewID()
	}
	if sh.Token == "" {
		sh.Token = RandomToken(18)
	}
	if sh.CreatedAt == "" {
		sh.CreatedAt = Now()
	}
	_, err := s.db.Exec(`INSERT INTO shares (id, token, name, path, owner_id, kind, password_hash,
		expires_at, download_only, max_downloads, downloads, size_bytes, created_at, last_access_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		sh.ID, sh.Token, sh.Name, sh.Path, sh.OwnerID, sh.Kind, sh.PasswordHash,
		nullIfEmpty(sh.ExpiresAt), sh.DownloadOnly, sh.MaxDownloads, sh.Downloads,
		sh.SizeBytes, sh.CreatedAt, nullIfEmpty(sh.LastAccessAt))
	return err
}

// GetShare returns a share by ID.
func (s *Store) GetShare(id string) (*Share, error) {
	sh, err := scanShare(s.db.QueryRow(`SELECT `+shareColumns+` FROM shares WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return sh, err
}

// GetShareByToken resolves a public share token.
func (s *Store) GetShareByToken(token string) (*Share, error) {
	sh, err := scanShare(s.db.QueryRow(`SELECT `+shareColumns+` FROM shares WHERE token=?`, token))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return sh, err
}

// ListShares lists shares, optionally only for one owner.
func (s *Store) ListShares(ownerID int64, all bool) ([]Share, error) {
	q := `SELECT ` + shareColumns + ` FROM shares`
	args := []any{}
	if !all {
		q += ` WHERE owner_id=?`
		args = append(args, ownerID)
	}
	q += ` ORDER BY created_at DESC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Share{}
	for rows.Next() {
		sh, err := scanShare(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sh)
	}
	return out, rows.Err()
}

// DeleteShare removes a share link.
func (s *Store) DeleteShare(id string) error {
	_, err := s.db.Exec(`DELETE FROM shares WHERE id=?`, id)
	return err
}

// TouchShare records an access and increments the download counter when needed.
func (s *Store) TouchShare(id string, countDownload bool) error {
	now := Now()
	if countDownload {
		_, err := s.db.Exec(`UPDATE shares SET downloads=downloads+1, last_access_at=? WHERE id=?`, now, id)
		return err
	}
	_, err := s.db.Exec(`UPDATE shares SET last_access_at=? WHERE id=?`, now, id)
	return err
}

// CountShares returns the number of shares owned by a user (0 = all).
func (s *Store) CountShares(ownerID int64) (int, error) {
	q := `SELECT COUNT(*) FROM shares`
	args := []any{}
	if ownerID > 0 {
		q += ` WHERE owner_id=?`
		args = append(args, ownerID)
	}
	var n int
	err := s.db.QueryRow(q, args...).Scan(&n)
	return n, err
}

// ---------- trash ----------

// AddTrashItem records a soft-deleted file.
func (s *Store) AddTrashItem(t *TrashItem) error {
	if t.ID == "" {
		t.ID = NewID()
	}
	if t.DeletedAt == "" {
		t.DeletedAt = Now()
	}
	_, err := s.db.Exec(`INSERT INTO trash_items (id, owner_id, original_path, trash_path, name, is_dir,
		size_bytes, deleted_at, expires_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		t.ID, t.OwnerID, t.OriginalPath, t.TrashPath, t.Name, t.IsDir,
		t.SizeBytes, t.DeletedAt, nullIfEmpty(t.ExpiresAt))
	return err
}

// ListTrash lists a user's trash (ownerID 0 = all users, admin view).
func (s *Store) ListTrash(ownerID int64) ([]TrashItem, error) {
	q := `SELECT id, owner_id, original_path, trash_path, name, is_dir, size_bytes, deleted_at,
		COALESCE(expires_at, '') FROM trash_items`
	args := []any{}
	if ownerID > 0 {
		q += ` WHERE owner_id=?`
		args = append(args, ownerID)
	}
	q += ` ORDER BY deleted_at DESC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TrashItem{}
	for rows.Next() {
		var t TrashItem
		if err := rows.Scan(&t.ID, &t.OwnerID, &t.OriginalPath, &t.TrashPath, &t.Name, &t.IsDir,
			&t.SizeBytes, &t.DeletedAt, &t.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetTrashItem returns one trash entry.
func (s *Store) GetTrashItem(id string) (*TrashItem, error) {
	var t TrashItem
	err := s.db.QueryRow(`SELECT id, owner_id, original_path, trash_path, name, is_dir, size_bytes,
		deleted_at, COALESCE(expires_at, '') FROM trash_items WHERE id=?`, id).
		Scan(&t.ID, &t.OwnerID, &t.OriginalPath, &t.TrashPath, &t.Name, &t.IsDir,
			&t.SizeBytes, &t.DeletedAt, &t.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// DeleteTrashItem removes a trash record.
func (s *Store) DeleteTrashItem(id string) error {
	_, err := s.db.Exec(`DELETE FROM trash_items WHERE id=?`, id)
	return err
}

// ExpiredTrash returns trash entries past their retention date.
func (s *Store) ExpiredTrash(now string) ([]TrashItem, error) {
	rows, err := s.db.Query(`SELECT id, owner_id, original_path, trash_path, name, is_dir, size_bytes,
		deleted_at, COALESCE(expires_at, '') FROM trash_items
		WHERE expires_at IS NOT NULL AND expires_at != '' AND expires_at < ?`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TrashItem{}
	for rows.Next() {
		var t TrashItem
		if err := rows.Scan(&t.ID, &t.OwnerID, &t.OriginalPath, &t.TrashPath, &t.Name, &t.IsDir,
			&t.SizeBytes, &t.DeletedAt, &t.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// TrashSize returns total trash bytes for a user (0 = all).
func (s *Store) TrashSize(ownerID int64) (int64, error) {
	q := `SELECT COALESCE(SUM(size_bytes),0) FROM trash_items`
	args := []any{}
	if ownerID > 0 {
		q += ` WHERE owner_id=?`
		args = append(args, ownerID)
	}
	var n int64
	err := s.db.QueryRow(q, args...).Scan(&n)
	return n, err
}
