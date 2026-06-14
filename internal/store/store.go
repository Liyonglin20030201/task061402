package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "dbinspect.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS inspections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			target_name TEXT NOT NULL,
			target_type TEXT NOT NULL,
			check_type TEXT NOT NULL,
			status TEXT NOT NULL,
			risk_score INTEGER DEFAULT 0,
			summary TEXT,
			details TEXT,
			started_at DATETIME NOT NULL,
			finished_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS operation_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			level TEXT NOT NULL,
			component TEXT NOT NULL,
			message TEXT NOT NULL,
			metadata TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS reports (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			format TEXT NOT NULL,
			file_path TEXT NOT NULL,
			file_size INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_inspections_run_id ON inspections(run_id)`,
		`CREATE INDEX IF NOT EXISTS idx_inspections_target ON inspections(target_name)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_run_id ON operation_logs(run_id)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_level ON operation_logs(level)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %s: %w", m[:50], err)
		}
	}

	return nil
}
