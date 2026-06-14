package report

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Liyonglin20030201/task061402/internal/store"
)

type Generator interface {
	Format() string
	Generate(inspections []*store.Inspection, runID string, outputDir string) (string, error)
}

func NewGenerator(format string) (Generator, error) {
	switch format {
	case "html":
		return &HTMLGenerator{}, nil
	case "json":
		return &JSONGenerator{}, nil
	case "csv":
		return &CSVGenerator{}, nil
	default:
		return nil, fmt.Errorf("unsupported report format: %s", format)
	}
}

func writeAtomic(filePath string, data []byte) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create report directory: %w", err)
	}

	tmpFile := filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpFile, filePath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to finalize report file: %w", err)
	}

	return nil
}
