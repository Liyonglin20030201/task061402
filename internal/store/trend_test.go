package store

import (
	"testing"
	"time"
)

func TestGetInspectionsByTimeRange(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	now := time.Now()

	inspections := []*Inspection{
		{RunID: "r1", TargetName: "db1", TargetType: "mysql", CheckType: "ping", Status: "success", RiskScore: 10, Summary: "ok", StartedAt: now.Add(-2 * time.Hour), FinishedAt: now.Add(-2 * time.Hour)},
		{RunID: "r2", TargetName: "db1", TargetType: "mysql", CheckType: "ping", Status: "warning", RiskScore: 30, Summary: "slow", StartedAt: now.Add(-1 * time.Hour), FinishedAt: now.Add(-1 * time.Hour)},
		{RunID: "r3", TargetName: "db2", TargetType: "postgres", CheckType: "ping", Status: "success", RiskScore: 0, Summary: "ok", StartedAt: now, FinishedAt: now},
	}
	for _, insp := range inspections {
		if err := s.SaveInspection(insp); err != nil {
			t.Fatalf("failed to save: %v", err)
		}
	}

	results, err := s.GetInspectionsByTimeRange("db1", now.Add(-3*time.Hour), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results for db1, got %d", len(results))
	}

	results, err = s.GetInspectionsByTimeRange("db2", now.Add(-3*time.Hour), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for db2, got %d", len(results))
	}
}

func TestGetInspectionsByCheckTypeAndTarget(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	now := time.Now()
	s.SaveInspection(&Inspection{RunID: "r1", TargetName: "db1", TargetType: "mysql", CheckType: "ping", Status: "success", RiskScore: 10, Summary: "ok", StartedAt: now, FinishedAt: now})
	s.SaveInspection(&Inspection{RunID: "r1", TargetName: "db1", TargetType: "mysql", CheckType: "capacity", Status: "success", RiskScore: 20, Summary: "ok", StartedAt: now, FinishedAt: now})
	s.SaveInspection(&Inspection{RunID: "r2", TargetName: "db1", TargetType: "mysql", CheckType: "ping", Status: "warning", RiskScore: 40, Summary: "slow", StartedAt: now, FinishedAt: now})

	results, err := s.GetInspectionsByCheckTypeAndTarget("db1", "ping", now.Add(-time.Hour), now.Add(time.Minute), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 ping results, got %d", len(results))
	}

	results, err = s.GetInspectionsByCheckTypeAndTarget("db1", "capacity", now.Add(-time.Hour), now.Add(time.Minute), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 capacity result, got %d", len(results))
	}
}

func TestGetAllTargetNames(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	now := time.Now()
	s.SaveInspection(&Inspection{RunID: "r1", TargetName: "alpha", TargetType: "mysql", CheckType: "ping", Status: "success", RiskScore: 0, Summary: "ok", StartedAt: now, FinishedAt: now})
	s.SaveInspection(&Inspection{RunID: "r1", TargetName: "beta", TargetType: "postgres", CheckType: "ping", Status: "success", RiskScore: 0, Summary: "ok", StartedAt: now, FinishedAt: now})
	s.SaveInspection(&Inspection{RunID: "r2", TargetName: "alpha", TargetType: "mysql", CheckType: "capacity", Status: "success", RiskScore: 0, Summary: "ok", StartedAt: now, FinishedAt: now})

	names, err := s.GetAllTargetNames()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("expected 2 distinct target names, got %d: %v", len(names), names)
	}
}
