package store

import (
	"encoding/json"
	"fmt"
	"time"
)

type OperationLog struct {
	ID        int64
	RunID     string
	Level     string
	Component string
	Message   string
	Metadata  map[string]interface{}
	CreatedAt time.Time
}

func (s *Store) SaveLog(log *OperationLog) error {
	var metaJSON string
	if log.Metadata != nil {
		data, _ := json.Marshal(log.Metadata)
		metaJSON = string(data)
	}

	_, err := s.db.Exec(
		`INSERT INTO operation_logs (run_id, level, component, message, metadata)
		 VALUES (?, ?, ?, ?, ?)`,
		log.RunID, log.Level, log.Component, log.Message, metaJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to save operation log: %w", err)
	}

	return nil
}

func (s *Store) GetLogsByRunID(runID string) ([]*OperationLog, error) {
	rows, err := s.db.Query(
		`SELECT id, run_id, level, component, message, metadata, created_at
		 FROM operation_logs WHERE run_id = ? ORDER BY created_at`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}
	defer rows.Close()

	var results []*OperationLog
	for rows.Next() {
		log := &OperationLog{}
		var metaStr string
		err := rows.Scan(&log.ID, &log.RunID, &log.Level, &log.Component, &log.Message, &metaStr, &log.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log row: %w", err)
		}
		if metaStr != "" {
			json.Unmarshal([]byte(metaStr), &log.Metadata)
		}
		results = append(results, log)
	}

	return results, rows.Err()
}

func (s *Store) SaveReport(runID, format, filePath string, fileSize int64) error {
	_, err := s.db.Exec(
		`INSERT INTO reports (run_id, format, file_path, file_size) VALUES (?, ?, ?, ?)`,
		runID, format, filePath, fileSize,
	)
	if err != nil {
		return fmt.Errorf("failed to save report: %w", err)
	}
	return nil
}
