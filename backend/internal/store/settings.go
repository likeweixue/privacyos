package store

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// GetSetting returns the raw JSON value for a key.
func (s *Store) GetSetting(key string) (json.RawMessage, bool, error) {
	var raw string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return json.RawMessage(raw), true, nil
}

// Setting unmarshals a setting into dst. Missing settings are left untouched.
func (s *Store) Setting(key string, dst any) (bool, error) {
	raw, ok, err := s.GetSetting(key)
	if err != nil || !ok {
		return ok, err
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return true, err
	}
	return true, nil
}

// SetSetting stores a JSON-encoded setting.
func (s *Store) SetSetting(key string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO settings (key, value, updated_at) VALUES (?,?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		key, string(raw), Now())
	return err
}

// AllSettings returns every setting as raw JSON.
func (s *Store) AllSettings() (map[string]json.RawMessage, error) {
	rows, err := s.db.Query(`SELECT key, value FROM settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]json.RawMessage{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = json.RawMessage(v)
	}
	return out, rows.Err()
}

// DefaultSetting writes a default value only when the key does not exist.
func (s *Store) DefaultSetting(key string, v any) error {
	_, ok, err := s.GetSetting(key)
	if err != nil || ok {
		return err
	}
	return s.SetSetting(key, v)
}
