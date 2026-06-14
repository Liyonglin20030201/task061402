package report

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/Liyonglin20030201/task061402/internal/store"
)

type CSVGenerator struct{}

func (c *CSVGenerator) Format() string { return "csv" }

func (c *CSVGenerator) Generate(inspections []*store.Inspection, runID string, outputDir string) (string, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	header := []string{"Target", "Type", "Check", "Status", "Risk Score", "Summary", "Started At", "Finished At"}
	if err := writer.Write(header); err != nil {
		return "", fmt.Errorf("failed to write CSV header: %w", err)
	}

	for _, insp := range inspections {
		row := []string{
			insp.TargetName,
			insp.TargetType,
			insp.CheckType,
			insp.Status,
			strconv.Itoa(insp.RiskScore),
			insp.Summary,
			insp.StartedAt.Format("2006-01-02 15:04:05"),
			insp.FinishedAt.Format("2006-01-02 15:04:05"),
		}
		if err := writer.Write(row); err != nil {
			return "", fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("CSV write error: %w", err)
	}

	filePath := filepath.Join(outputDir, fmt.Sprintf("report_%s.csv", runID[:8]))
	if err := writeAtomic(filePath, buf.Bytes()); err != nil {
		return "", err
	}

	return filePath, nil
}
