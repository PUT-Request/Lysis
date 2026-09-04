package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000&_fk=true")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	return db, nil
}

func Migrate(db *sql.DB) error {
	for _, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, stmt)
		}
	}
	return nil
}

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS users (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		email      TEXT UNIQUE NOT NULL,
		password   TEXT NOT NULL,
		created_at TEXT DEFAULT (datetime('now'))
	)`,

	`CREATE TABLE IF NOT EXISTS scans (
		id               TEXT PRIMARY KEY,
		user_id          INTEGER NOT NULL REFERENCES users(id),
		type             TEXT NOT NULL CHECK(type IN ('exploit','malware')),
		source           TEXT NOT NULL CHECK(source IN ('upload','github')),
		source_detail    TEXT,
		status           TEXT NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','running','completed','failed')),
		verdict          TEXT,
		file_hash        TEXT,
		prescan_json     TEXT,
		result_json      TEXT,
		share_visibility TEXT NOT NULL DEFAULT 'private' CHECK(share_visibility IN ('private','logged_in','public')),
		share_token      TEXT UNIQUE,
		error_message    TEXT,
		created_at       TEXT DEFAULT (datetime('now')),
		completed_at     TEXT
	)`,

	`CREATE TABLE IF NOT EXISTS hash_cache (
		hash               TEXT PRIMARY KEY,
		virustotal_result  TEXT,
		abusech_result     TEXT,
		classification     TEXT,
		cached_at          TEXT DEFAULT (datetime('now'))
	)`,

	`CREATE INDEX IF NOT EXISTS idx_scans_user_id ON scans(user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_scans_status ON scans(status)`,
	`CREATE INDEX IF NOT EXISTS idx_scans_share_token ON scans(share_token)`,
	`CREATE INDEX IF NOT EXISTS idx_hash_cache_lookup ON hash_cache(hash)`,
}
