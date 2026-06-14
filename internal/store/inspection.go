package store

import (
	"encoding/json"
	"fmt"
	"time"
)

type Inspection struct {
	ID         int64
	RunID      string
	TargetName string
	TargetType string
	CheckType  string
	Status     string
	RiskScore  int
	Summary    string
	Details    map[string]interface{}
	StartedAt  time.Time
	FinishedAt time.Time
	CreatedAt  time.Time
}

func (s *Store) SaveInspection(insp *Inspection) error {
	detailsJSON, err := json.Marshal(insp.Details)
	if err != nil {
		return fmt.Errorf("failed to marshal details: %w", err)
	}

	_, err = s.db.Exec(
		`INSERT INTO inspections (run_id, target_name, target_type, check_type, status, risk_score, summary, details, started_at, finished_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		insp.RunID, insp.TargetName, insp.TargetType, insp.CheckType,
		insp.Status, insp.RiskScore, insp.Summary, string(detailsJSON),
		insp.StartedAt, insp.FinishedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save inspection: %w", err)
	}

	return nil
}

func (s *Store) GetInspectionsByRunID(runID string) ([]*Inspection, error) {
	rows, err := s.db.Query(
		`SELECT id, run_id, target_name, target_type, check_type, status, risk_score, summary, details, started_at, finished_at, created_at
		 FROM inspections WHERE run_id = ? ORDER BY started_at`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query inspections: %w", err)
	}
	defer rows.Close()

	var results []*Inspection
	for rows.Next() {
		insp := &Inspection{}
		var detailsStr string
		err := rows.Scan(
			&insp.ID, &insp.RunID, &insp.TargetName, &insp.TargetType,
			&insp.CheckType, &insp.Status, &insp.RiskScore, &insp.Summary,
			&detailsStr, &insp.StartedAt, &insp.FinishedAt, &insp.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan inspection row: %w", err)
		}
		if detailsStr != "" {
			json.Unmarshal([]byte(detailsStr), &insp.Details)
		}
		results = append(results, insp)
	}

	return results, rows.Err()
}

func (s *Store) GetLatestInspections(targetName string, limit int) ([]*Inspection, error) {
	rows, err := s.db.Query(
		`SELECT id, run_id, target_name, target_type, check_type, status, risk_score, summary, details, started_at, finished_at, created_at
		 FROM inspections WHERE target_name = ? ORDER BY created_at DESC LIMIT ?`,
		targetName, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query inspections: %w", err)
	}
	defer rows.Close()

	var results []*Inspection
	for rows.Next() {
		insp := &Inspection{}
		var detailsStr string
		err := rows.Scan(
			&insp.ID, &insp.RunID, &insp.TargetName, &insp.TargetType,
			&insp.CheckType, &insp.Status, &insp.RiskScore, &insp.Summary,
			&detailsStr, &insp.StartedAt, &insp.FinishedAt, &insp.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan inspection row: %w", err)
		}
		if detailsStr != "" {
			json.Unmarshal([]byte(detailsStr), &insp.Details)
		}
		results = append(results, insp)
	}

	return results, rows.Err()
}
