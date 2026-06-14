package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

func (s *Store) GetInspectionsByTimeRange(targetName string, start, end time.Time) ([]*Inspection, error) {
	query := `SELECT id, run_id, target_name, target_type, check_type, status, risk_score,
		summary, details, started_at, finished_at, created_at
		FROM inspections WHERE target_name = ? AND created_at >= ? AND created_at <= ?
		ORDER BY created_at DESC`

	rows, err := s.db.Query(query, targetName, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query inspections by time range: %w", err)
	}
	defer rows.Close()

	return scanInspectionRows(rows)
}

func (s *Store) GetInspectionsByCheckTypeAndTarget(targetName, checkType string, start, end time.Time, limit int) ([]*Inspection, error) {
	query := `SELECT id, run_id, target_name, target_type, check_type, status, risk_score,
		summary, details, started_at, finished_at, created_at
		FROM inspections WHERE target_name = ? AND check_type = ? AND created_at >= ? AND created_at <= ?
		ORDER BY created_at DESC LIMIT ?`

	rows, err := s.db.Query(query, targetName, checkType, start, end, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query inspections by check type: %w", err)
	}
	defer rows.Close()

	return scanInspectionRows(rows)
}

func (s *Store) GetAllTargetNames() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT target_name FROM inspections ORDER BY target_name`)
	if err != nil {
		return nil, fmt.Errorf("failed to query target names: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

func (s *Store) GetCheckTypesForTarget(targetName string) ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT check_type FROM inspections WHERE target_name = ? ORDER BY check_type`, targetName)
	if err != nil {
		return nil, fmt.Errorf("failed to query check types: %w", err)
	}
	defer rows.Close()

	var types []string
	for rows.Next() {
		var ct string
		if err := rows.Scan(&ct); err != nil {
			continue
		}
		types = append(types, ct)
	}
	return types, nil
}

func scanInspectionRows(rows *sql.Rows) ([]*Inspection, error) {
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
