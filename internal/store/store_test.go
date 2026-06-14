package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreNew(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	dbPath := filepath.Join(tmpDir, "dbinspect.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("expected database file to be created")
	}
}

func TestSaveAndGetInspection(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	insp := &Inspection{
		RunID:      "test-run-123",
		TargetName: "test-db",
		TargetType: "mysql",
		CheckType:  "ping",
		Status:     "success",
		RiskScore:  0,
		Summary:    "connected in 5ms",
		Details:    map[string]interface{}{"latency_ms": 5},
		StartedAt:  time.Now(),
		FinishedAt: time.Now(),
	}

	if err := s.SaveInspection(insp); err != nil {
		t.Fatalf("failed to save inspection: %v", err)
	}

	results, err := s.GetInspectionsByRunID("test-run-123")
	if err != nil {
		t.Fatalf("failed to get inspections: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Summary != "connected in 5ms" {
		t.Errorf("expected summary 'connected in 5ms', got %q", results[0].Summary)
	}
}

func TestSaveLog(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	log := &OperationLog{
		RunID:     "test-run-123",
		Level:     "info",
		Component: "ping",
		Message:   "testing connection",
		Metadata:  map[string]interface{}{"target": "db1"},
	}

	if err := s.SaveLog(log); err != nil {
		t.Fatalf("failed to save log: %v", err)
	}

	logs, err := s.GetLogsByRunID("test-run-123")
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}

	if logs[0].Message != "testing connection" {
		t.Errorf("expected message 'testing connection', got %q", logs[0].Message)
	}
}

func TestSaveReport(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	err = s.SaveReport("run-1", "html", "/reports/report.html", 1024)
	if err != nil {
		t.Fatalf("failed to save report: %v", err)
	}
}
