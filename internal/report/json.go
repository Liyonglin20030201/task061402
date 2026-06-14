package report

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Liyonglin20030201/task061402/internal/store"
)

type JSONGenerator struct{}

func (j *JSONGenerator) Format() string { return "json" }

func (j *JSONGenerator) Generate(inspections []*store.Inspection, runID string, outputDir string) (string, error) {
	data := struct {
		RunID       string              `json:"run_id"`
		GeneratedAt string              `json:"generated_at"`
		Summary     reportSummary       `json:"summary"`
		Inspections []*store.Inspection `json:"inspections"`
	}{
		RunID:       runID,
		GeneratedAt: time.Now().Format(time.RFC3339),
		Summary:     computeSummary(inspections),
		Inspections: inspections,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON report: %w", err)
	}

	filePath := filepath.Join(outputDir, fmt.Sprintf("report_%s.json", runID[:8]))
	if err := writeAtomic(filePath, jsonData); err != nil {
		return "", err
	}

	return filePath, nil
}
