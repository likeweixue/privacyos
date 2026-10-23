package store

import (
	"strings"
)

// AuditQuery filters audit records.
type AuditQuery struct {
	Category string
	Username string
	Action   string
	From     string
	To       string
	Page     int
	PageSize int
}

// Audit inserts an audit record.
func (s *Store) Audit(e *AuditLog) error {
	if e.CreatedAt == "" {
		e.CreatedAt = Now()
	}
	if e.Result == "" {
		e.Result = "ok"
	}
	_, err := s.db.Exec(`INSERT INTO audit_logs
		(created_at, user_id, username, category, action, target, detail, ip, result)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		e.CreatedAt, nullableID(e.UserID), e.Username, e.Category, e.Action,
		e.Target, e.Detail, e.IP, e.Result)
	return err
}

// QueryAudit returns a filtered page of audit records plus the total count.
func (s *Store) QueryAudit(q AuditQuery) ([]AuditLog, int, error) {
	if q.PageSize <= 0 || q.PageSize > 500 {
		q.PageSize = 50
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	var where []string
	var args []any
	if q.Category != "" && q.Category != "all" {
		where = append(where, "category = ?")
		args = append(args, q.Category)
	}
	if q.Username != "" {
		where = append(where, "username LIKE ? ESCAPE '\\'")
		args = append(args, like(q.Username))
	}
	if q.Action != "" {
		where = append(where, "action LIKE ? ESCAPE '\\'")
		args = append(args, like(q.Action))
	}
	if q.From != "" {
		where = append(where, "created_at >= ?")
		args = append(args, q.From)
	}
	if q.To != "" {
		where = append(where, "created_at <= ?")
		args = append(args, q.To)
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM audit_logs`+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, q.PageSize, (q.Page-1)*q.PageSize)
	rows, err := s.db.Query(`SELECT id, created_at, COALESCE(user_id,0), username, category, action,
		target, detail, ip, result FROM audit_logs`+clause+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []AuditLog{}
	for rows.Next() {
		var e AuditLog
		if err := rows.Scan(&e.ID, &e.CreatedAt, &e.UserID, &e.Username, &e.Category, &e.Action,
			&e.Target, &e.Detail, &e.IP, &e.Result); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

// RecentAudit returns the newest audit entries, optionally filtered by category.
func (s *Store) RecentAudit(limit int, categories ...string) ([]AuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	q := `SELECT id, created_at, COALESCE(user_id,0), username, category, action, target, detail, ip, result
		FROM audit_logs`
	args := []any{}
	if len(categories) > 0 {
		ph := make([]string, len(categories))
		for i, c := range categories {
			ph[i] = "?"
			args = append(args, c)
		}
		q += ` WHERE category IN (` + strings.Join(ph, ",") + `)`
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditLog{}
	for rows.Next() {
		var e AuditLog
		if err := rows.Scan(&e.ID, &e.CreatedAt, &e.UserID, &e.Username, &e.Category, &e.Action,
			&e.Target, &e.Detail, &e.IP, &e.Result); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// AuditCounts returns counts per category for today.
func (s *Store) AuditCounts() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT category, COUNT(*) FROM audit_logs GROUP BY category`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var cat string
		var n int
		if err := rows.Scan(&cat, &n); err != nil {
			return nil, err
		}
		out[cat] = n
	}
	return out, rows.Err()
}

// DeleteAuditBefore removes audit entries older than the timestamp.
func (s *Store) DeleteAuditBefore(ts string) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM audit_logs WHERE created_at < ?`, ts)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func nullableID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}
