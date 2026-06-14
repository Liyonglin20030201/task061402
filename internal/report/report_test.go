package report

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Liyonglin20030201/task061402/internal/store"
)

func testInspections() []*store.Inspection {
	now := time.Now()
	return []*store.Inspection{
		{
			ID: 1, RunID: "test-run-001", TargetName: "prod-mysql", TargetType: "mysql",
			CheckType: "ping", Status: "success", RiskScore: 0,
			Summary: "connected in 5ms", StartedAt: now, FinishedAt: now,
		},
		{
			ID: 2, RunID: "test-run-001", TargetName: "prod-mysql", TargetType: "mysql",
			CheckType: "slowquery", Status: "warning", RiskScore: 40,
			Summary: "3 slow queries detected", StartedAt: now, FinishedAt: now,
		},
		{
			ID: 3, RunID: "test-run-001", TargetName: "prod-redis", TargetType: "redis",
			CheckType: "ping", Status: "success", RiskScore: 0,
			Summary: "connected in 2ms", StartedAt: now, FinishedAt: now,
		},
	}
}

func TestHTMLGenerator(t *testing.T) {
	tmpDir := t.TempDir()
	gen := &HTMLGenerator{}

	filePath, err := gen.Generate(testInspections(), "test-run-001-abcd-efgh", tmpDir)
	if err != nil {
		t.Fatalf("HTML generation failed: %v", err)
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("expected file %s to exist", filePath)
	}

	content, _ := os.ReadFile(filePath)
	if len(content) == 0 {
		t.Error("expected non-empty HTML file")
	}
}

func TestJSONGenerator(t *testing.T) {
	tmpDir := t.TempDir()
	gen := &JSONGenerator{}

	filePath, err := gen.Generate(testInspections(), "test-run-001-abcd-efgh", tmpDir)
	if err != nil {
		t.Fatalf("JSON generation failed: %v", err)
	}

	content, _ := os.ReadFile(filePath)
	if len(content) == 0 {
		t.Error("expected non-empty JSON file")
	}

	if filepath.Ext(filePath) != ".json" {
		t.Errorf("expected .json extension, got %s", filepath.Ext(filePath))
	}
}

func TestCSVGenerator(t *testing.T) {
	tmpDir := t.TempDir()
	gen := &CSVGenerator{}

	filePath, err := gen.Generate(testInspections(), "test-run-001-abcd-efgh", tmpDir)
	if err != nil {
		t.Fatalf("CSV generation failed: %v", err)
	}

	content, _ := os.ReadFile(filePath)
	if len(content) == 0 {
		t.Error("expected non-empty CSV file")
	}
}

func TestNewGenerator(t *testing.T) {
	tests := []struct {
		format  string
		wantErr bool
	}{
		{"html", false},
		{"json", false},
		{"csv", false},
		{"pdf", true},
		{"", true},
	}

	for _, tt := range tests {
		gen, err := NewGenerator(tt.format)
		if tt.wantErr && err == nil {
			t.Errorf("NewGenerator(%q): expected error", tt.format)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("NewGenerator(%q): unexpected error: %v", tt.format, err)
		}
		if !tt.wantErr && gen == nil {
			t.Errorf("NewGenerator(%q): expected non-nil generator", tt.format)
		}
	}
}
