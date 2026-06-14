package inspector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Liyonglin20030201/task061402/internal/config"
	"github.com/Liyonglin20030201/task061402/internal/connector"
)

type BackupInspector struct{}

func NewBackupInspector() *BackupInspector { return &BackupInspector{} }

func (b *BackupInspector) Name() string { return "backup" }

func (b *BackupInspector) Run(ctx context.Context, conn connector.Connector, cfg *config.Config) (*Result, error) {
	result := NewResult("backup")

	if !cfg.Checks.Backup.Enabled {
		return result.Finish(StatusSkipped, "backup check disabled"), nil
	}

	if len(cfg.Checks.Backup.Paths) == 0 {
		return result.Finish(StatusSkipped, "no backup paths configured"), nil
	}

	var findings []map[string]interface{}
	var issues int

	for _, backupPath := range cfg.Checks.Backup.Paths {
		info, err := os.Stat(backupPath)
		if err != nil {
			issues++
			findings = append(findings, map[string]interface{}{
				"path":   backupPath,
				"status": "missing",
				"error":  err.Error(),
			})
			continue
		}

		if !info.IsDir() {
			age := time.Since(info.ModTime())
			finding := map[string]interface{}{
				"path":     backupPath,
				"size":     info.Size(),
				"modified": info.ModTime().Format(time.RFC3339),
				"age":      age.String(),
			}
			if age > cfg.Checks.Backup.MaxAge {
				issues++
				finding["status"] = "stale"
			} else {
				finding["status"] = "ok"
			}
			findings = append(findings, finding)
			continue
		}

		entries, err := os.ReadDir(backupPath)
		if err != nil {
			issues++
			findings = append(findings, map[string]interface{}{
				"path":   backupPath,
				"status": "unreadable",
				"error":  err.Error(),
			})
			continue
		}

		if len(entries) == 0 {
			issues++
			findings = append(findings, map[string]interface{}{
				"path":   backupPath,
				"status": "empty",
			})
			continue
		}

		var latestTime time.Time
		var latestFile string
		var latestSize int64
		for _, entry := range entries {
			entryInfo, err := entry.Info()
			if err != nil {
				continue
			}
			if entryInfo.ModTime().After(latestTime) {
				latestTime = entryInfo.ModTime()
				latestFile = filepath.Join(backupPath, entry.Name())
				latestSize = entryInfo.Size()
			}
		}

		age := time.Since(latestTime)
		finding := map[string]interface{}{
			"path":        backupPath,
			"latest_file": latestFile,
			"latest_size": latestSize,
			"latest_time": latestTime.Format(time.RFC3339),
			"age":         age.String(),
			"file_count":  len(entries),
		}

		if age > cfg.Checks.Backup.MaxAge {
			issues++
			finding["status"] = "stale"
		} else if latestSize == 0 {
			issues++
			finding["status"] = "empty_file"
		} else {
			finding["status"] = "ok"
		}
		findings = append(findings, finding)
	}

	result.Details["findings"] = findings
	result.Details["issues"] = issues
	result.Details["paths_checked"] = len(cfg.Checks.Backup.Paths)

	if issues == 0 {
		result.RiskScore = 0
		return result.Finish(StatusSuccess, fmt.Sprintf("all %d backup paths verified", len(cfg.Checks.Backup.Paths))), nil
	}

	result.RiskScore = min(issues*30, 100)
	return result.Finish(StatusWarning, fmt.Sprintf("%d backup issues found across %d paths", issues, len(cfg.Checks.Backup.Paths))), nil
}
