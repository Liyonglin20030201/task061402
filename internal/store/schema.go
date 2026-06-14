package store

import (
	"encoding/json"
	"fmt"
	"time"
)

type SchemaSnapshot struct {
	ID           int64     `json:"id"`
	TargetName   string    `json:"target_name"`
	TargetType   string    `json:"target_type"`
	DatabaseName string    `json:"database_name"`
	SnapshotData string    `json:"snapshot_data"`
	RunID        string    `json:"run_id"`
	CreatedAt    time.Time `json:"created_at"`
}

func (s *Store) SaveSchemaSnapshot(snapshot *SchemaSnapshot) error {
	_, err := s.db.Exec(
		`INSERT INTO schema_snapshots (target_name, target_type, database_name, snapshot_data, run_id)
		 VALUES (?, ?, ?, ?, ?)`,
		snapshot.TargetName, snapshot.TargetType, snapshot.DatabaseName, snapshot.SnapshotData, snapshot.RunID,
	)
	if err != nil {
		return fmt.Errorf("failed to save schema snapshot: %w", err)
	}
	return nil
}

func (s *Store) GetLatestSchemaSnapshot(targetName, databaseName string) (*SchemaSnapshot, error) {
	row := s.db.QueryRow(
		`SELECT id, target_name, target_type, database_name, snapshot_data, run_id, created_at
		 FROM schema_snapshots
		 WHERE target_name = ? AND database_name = ?
		 ORDER BY created_at DESC LIMIT 1`,
		targetName, databaseName,
	)

	snap := &SchemaSnapshot{}
	err := row.Scan(&snap.ID, &snap.TargetName, &snap.TargetType, &snap.DatabaseName,
		&snap.SnapshotData, &snap.RunID, &snap.CreatedAt)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get latest schema snapshot: %w", err)
	}
	return snap, nil
}

func MarshalSchemaSnapshot(data interface{}) (string, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal schema snapshot: %w", err)
	}
	return string(b), nil
}

func UnmarshalSchemaSnapshot(data string, target interface{}) error {
	if err := json.Unmarshal([]byte(data), target); err != nil {
		return fmt.Errorf("failed to unmarshal schema snapshot: %w", err)
	}
	return nil
}
