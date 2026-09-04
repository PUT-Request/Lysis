package db

import (
	"database/sql"
	"time"
)

type User struct {
	ID        int    `json:"id"`
	Email     string `json:"email"`
	Password  string `json:"-"`
	CreatedAt string `json:"created_at"`
}

type Scan struct {
	ID              string  `json:"id"`
	UserID          int     `json:"user_id"`
	Type            string  `json:"type"`
	Source          string  `json:"source"`
	SourceDetail    *string `json:"source_detail,omitempty"`
	Status          string  `json:"status"`
	Verdict         *string `json:"verdict,omitempty"`
	FileHash        *string `json:"file_hash,omitempty"`
	PrescanJSON     *string `json:"prescan_json,omitempty"`
	ResultJSON      *string `json:"result_json,omitempty"`
	ShareVisibility string  `json:"share_visibility"`
	ShareToken      *string `json:"share_token,omitempty"`
	ErrorMessage    *string `json:"error_message,omitempty"`
	CreatedAt       string  `json:"created_at"`
	CompletedAt     *string `json:"completed_at,omitempty"`
}

type HashCache struct {
	Hash             string  `json:"hash"`
	VirusTotalResult *string `json:"virustotal_result,omitempty"`
	AbuseChResult    *string `json:"abusech_result,omitempty"`
	Classification   *string `json:"classification,omitempty"`
	CachedAt         string  `json:"cached_at"`
}

type ScanListParams struct {
	UserID int
	Page   int
	Limit  int
	Search string
}

type PaginatedScans struct {
	Scans      []Scan `json:"scans"`
	Total      int    `json:"total"`
	Page       int    `json:"page"`
	TotalPages int    `json:"total_pages"`
}

type Stats struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Running   int `json:"running"`
	Queued    int `json:"queued"`
	Failed    int `json:"failed"`
}

func InsertUser(db *sql.DB, email, passwordHash string) (*User, error) {
	res, err := db.Exec("INSERT INTO users (email, password) VALUES (?, ?)", email, passwordHash)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &User{ID: int(id), Email: email, CreatedAt: time.Now().UTC().Format(time.RFC3339)}, nil
}

func GetUserByEmail(db *sql.DB, email string) (*User, error) {
	u := &User{}
	err := db.QueryRow("SELECT id, email, password, created_at FROM users WHERE email = ?", email).
		Scan(&u.ID, &u.Email, &u.Password, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func GetUserByID(db *sql.DB, id int) (*User, error) {
	u := &User{}
	err := db.QueryRow("SELECT id, email, password, created_at FROM users WHERE id = ?", id).
		Scan(&u.ID, &u.Email, &u.Password, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func InsertScan(db *sql.DB, s *Scan) error {
	_, err := db.Exec(`INSERT INTO scans (id, user_id, type, source, source_detail, status, file_hash, share_visibility)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.UserID, s.Type, s.Source, s.SourceDetail, s.Status, s.FileHash, s.ShareVisibility)
	return err
}

func UpdateScanStatus(db *sql.DB, id, status, errMsg string) error {
	if errMsg != "" {
		_, err := db.Exec(`UPDATE scans SET status = ?, error_message = ?, completed_at = datetime('now') WHERE id = ?`,
			status, errMsg, id)
		return err
	}
	_, err := db.Exec(`UPDATE scans SET status = ?, completed_at = datetime('now') WHERE id = ?`, status, id)
	return err
}

func UpdateScanRunning(db *sql.DB, id string) error {
	_, err := db.Exec(`UPDATE scans SET status = 'running' WHERE id = ?`, id)
	return err
}

func UpdateScanResult(db *sql.DB, id string, verdict string, prescanJSON, resultJSON string) error {
	_, err := db.Exec(`UPDATE scans SET status = 'completed', verdict = ?, prescan_json = ?, result_json = ?, completed_at = datetime('now') WHERE id = ?`,
		verdict, prescanJSON, resultJSON, id)
	return err
}

func GetScanByID(db *sql.DB, id string) (*Scan, error) {
	s := &Scan{}
	err := db.QueryRow(`SELECT id, user_id, type, source, source_detail, status, verdict,
		file_hash, prescan_json, result_json, share_visibility, share_token, error_message, created_at, completed_at
		FROM scans WHERE id = ?`, id).
		Scan(&s.ID, &s.UserID, &s.Type, &s.Source, &s.SourceDetail, &s.Status, &s.Verdict,
			&s.FileHash, &s.PrescanJSON, &s.ResultJSON, &s.ShareVisibility, &s.ShareToken,
			&s.ErrorMessage, &s.CreatedAt, &s.CompletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func GetScanByShareToken(db *sql.DB, token string) (*Scan, error) {
	s := &Scan{}
	err := db.QueryRow(`SELECT id, user_id, type, source, source_detail, status, verdict,
		file_hash, prescan_json, result_json, share_visibility, share_token, error_message, created_at, completed_at
		FROM scans WHERE share_token = ?`, token).
		Scan(&s.ID, &s.UserID, &s.Type, &s.Source, &s.SourceDetail, &s.Status, &s.Verdict,
			&s.FileHash, &s.PrescanJSON, &s.ResultJSON, &s.ShareVisibility, &s.ShareToken,
			&s.ErrorMessage, &s.CreatedAt, &s.CompletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func ListScans(db *sql.DB, params ScanListParams) (*PaginatedScans, error) {
	var total int
	countQuery := "SELECT COUNT(*) FROM scans WHERE user_id = ?"
	var countArgs []interface{}
	countArgs = append(countArgs, params.UserID)

	if params.Search != "" {
		countQuery += " AND (source_detail LIKE ? OR file_hash LIKE ? OR id LIKE ?)"
		searchPattern := "%" + params.Search + "%"
		countArgs = append(countArgs, searchPattern, searchPattern, searchPattern)
	}

	if err := db.QueryRow(countQuery, countArgs...).Scan(&total); err != nil {
		return nil, err
	}

	offset := (params.Page - 1) * params.Limit
	totalPages := (total + params.Limit - 1) / params.Limit

	query := `SELECT id, user_id, type, source, source_detail, status, verdict,
		file_hash, prescan_json, result_json, share_visibility, share_token, error_message, created_at, completed_at
		FROM scans WHERE user_id = ?`
	var args []interface{}
	args = append(args, params.UserID)

	if params.Search != "" {
		query += " AND (source_detail LIKE ? OR file_hash LIKE ? OR id LIKE ?)"
		searchPattern := "%" + params.Search + "%"
		args = append(args, searchPattern, searchPattern, searchPattern)
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, params.Limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scans []Scan
	for rows.Next() {
		var s Scan
		if err := rows.Scan(&s.ID, &s.UserID, &s.Type, &s.Source, &s.SourceDetail, &s.Status, &s.Verdict,
			&s.FileHash, &s.PrescanJSON, &s.ResultJSON, &s.ShareVisibility, &s.ShareToken,
			&s.ErrorMessage, &s.CreatedAt, &s.CompletedAt); err != nil {
			return nil, err
		}
		scans = append(scans, s)
	}

	if scans == nil {
		scans = []Scan{}
	}

	return &PaginatedScans{
		Scans:      scans,
		Total:      total,
		Page:       params.Page,
		TotalPages: totalPages,
	}, nil
}

func UpdateShareVisibility(db *sql.DB, id, visibility string, token *string) error {
	_, err := db.Exec(`UPDATE scans SET share_visibility = ?, share_token = ? WHERE id = ?`,
		visibility, token, id)
	return err
}

func GetUserScansCount(db *sql.DB, userID int, since string) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM scans WHERE user_id = ? AND created_at >= ?",
		userID, since).Scan(&count)
	return count, err
}

func GetActiveScansCount(db *sql.DB, userID int) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM scans WHERE user_id = ? AND status IN ('queued','running')",
		userID).Scan(&count)
	return count, err
}

func GetGlobalActiveScansCount(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM scans WHERE status IN ('queued','running')").Scan(&count)
	return count, err
}

func GetUserStats(db *sql.DB, userID int) (*Stats, error) {
	s := &Stats{}
	row := db.QueryRow("SELECT "+
		"COUNT(*), "+
		"COUNT(CASE WHEN status = 'completed' THEN 1 END), "+
		"COUNT(CASE WHEN status = 'running' THEN 1 END), "+
		"COUNT(CASE WHEN status = 'queued' THEN 1 END), "+
		"COUNT(CASE WHEN status = 'failed' THEN 1 END) "+
		"FROM scans WHERE user_id = ?",
		userID)
	if err := row.Scan(&s.Total, &s.Completed, &s.Running, &s.Queued, &s.Failed); err != nil {
		return nil, err
	}
	return s, nil
}

func GetHashCache(db *sql.DB, hash string) (*HashCache, error) {
	h := &HashCache{}
	err := db.QueryRow("SELECT hash, virustotal_result, abusech_result, classification, cached_at FROM hash_cache WHERE hash = ?", hash).
		Scan(&h.Hash, &h.VirusTotalResult, &h.AbuseChResult, &h.Classification, &h.CachedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return h, nil
}

func SetHashCache(db *sql.DB, h *HashCache) error {
	_, err := db.Exec(`INSERT OR REPLACE INTO hash_cache (hash, virustotal_result, abusech_result, classification, cached_at)
		VALUES (?, ?, ?, ?, datetime('now'))`,
		h.Hash, h.VirusTotalResult, h.AbuseChResult, h.Classification)
	return err
}

func DeleteScan(db *sql.DB, id string) error {
	_, err := db.Exec("DELETE FROM scans WHERE id = ?", id)
	return err
}
