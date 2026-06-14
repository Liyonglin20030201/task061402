package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndGetSchemaSnapshot(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	snap := &SchemaSnapshot{
		TargetName:   "test-mysql",
		TargetType:   "mysql",
		DatabaseName: "mydb",
		SnapshotData: `{"tables":[{"name":"users"}]}`,
		RunID:        "run-123",
	}

	if err := s.SaveSchemaSnapshot(snap); err != nil {
		t.Fatalf("failed to save schema snapshot: %v", err)
	}

	got, err := s.GetLatestSchemaSnapshot("test-mysql", "mydb")
	if err != nil {
		t.Fatalf("failed to get schema snapshot: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if got.TargetName != "test-mysql" {
		t.Errorf("expected target_name 'test-mysql', got %q", got.TargetName)
	}
	if got.SnapshotData != `{"tables":[{"name":"users"}]}` {
		t.Errorf("unexpected snapshot_data: %q", got.SnapshotData)
	}
}

func TestGetLatestSchemaSnapshot_Empty(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	got, err := s.GetLatestSchemaSnapshot("nonexistent", "nodb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for non-existent snapshot, got %+v", got)
	}
}

func TestSchemaSnapshotTableExists(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	dbPath := filepath.Join(dir, "dbinspect.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("database file should exist")
	}

	var tableName string
	err = s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='schema_snapshots'`).Scan(&tableName)
	if err != nil {
		t.Fatalf("schema_snapshots table should exist: %v", err)
	}
}
