package report

import (
	"bytes"
	"fmt"
	"html/template"
	"path/filepath"
	"time"

	"github.com/Liyonglin20030201/task061402/internal/store"
)

type HTMLGenerator struct{}

func (h *HTMLGenerator) Format() string { return "html" }

func (h *HTMLGenerator) Generate(inspections []*store.Inspection, runID string, outputDir string) (string, error) {
	data := struct {
		RunID       string
		GeneratedAt string
		Inspections []*store.Inspection
		Summary     reportSummary
	}{
		RunID:       runID,
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		Inspections: inspections,
		Summary:     computeSummary(inspections),
	}

	tmpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render HTML template: %w", err)
	}

	filePath := filepath.Join(outputDir, fmt.Sprintf("report_%s.html", runID[:8]))
	if err := writeAtomic(filePath, buf.Bytes()); err != nil {
		return "", err
	}

	return filePath, nil
}

type reportSummary struct {
	Total    int
	Success  int
	Warning  int
	Error    int
	Skipped  int
	AvgRisk  int
}

func computeSummary(inspections []*store.Inspection) reportSummary {
	s := reportSummary{Total: len(inspections)}
	totalRisk := 0
	riskCount := 0
	for _, insp := range inspections {
		switch insp.Status {
		case "success":
			s.Success++
		case "warning":
			s.Warning++
		case "error":
			s.Error++
		case "skipped":
			s.Skipped++
		}
		if insp.Status != "skipped" {
			totalRisk += insp.RiskScore
			riskCount++
		}
	}
	if riskCount > 0 {
		s.AvgRisk = totalRisk / riskCount
	}
	return s
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Database Inspection Report - {{.RunID}}</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 20px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; }
        .header { background: #1a1a2e; color: white; padding: 30px; border-radius: 8px; margin-bottom: 20px; }
        .header h1 { margin: 0 0 10px 0; }
        .header p { margin: 0; opacity: 0.8; }
        .summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 15px; margin-bottom: 20px; }
        .summary-card { background: white; padding: 20px; border-radius: 8px; text-align: center; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .summary-card .number { font-size: 2em; font-weight: bold; }
        .summary-card .label { color: #666; font-size: 0.9em; }
        .success .number { color: #27ae60; }
        .warning .number { color: #f39c12; }
        .error .number { color: #e74c3c; }
        .table-container { background: white; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        table { width: 100%; border-collapse: collapse; }
        th { background: #2c3e50; color: white; padding: 12px 15px; text-align: left; }
        td { padding: 10px 15px; border-bottom: 1px solid #eee; }
        tr:hover { background: #f8f9fa; }
        .status-success { color: #27ae60; font-weight: bold; }
        .status-warning { color: #f39c12; font-weight: bold; }
        .status-error { color: #e74c3c; font-weight: bold; }
        .status-skipped { color: #95a5a6; font-weight: bold; }
        .risk-bar { width: 100px; height: 8px; background: #eee; border-radius: 4px; overflow: hidden; display: inline-block; }
        .risk-fill { height: 100%; border-radius: 4px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Database Inspection Report</h1>
            <p>Run ID: {{.RunID}} | Generated: {{.GeneratedAt}}</p>
        </div>

        <div class="summary">
            <div class="summary-card success"><div class="number">{{.Summary.Success}}</div><div class="label">Success</div></div>
            <div class="summary-card warning"><div class="number">{{.Summary.Warning}}</div><div class="label">Warning</div></div>
            <div class="summary-card error"><div class="number">{{.Summary.Error}}</div><div class="label">Error</div></div>
            <div class="summary-card"><div class="number">{{.Summary.AvgRisk}}</div><div class="label">Avg Risk Score</div></div>
        </div>

        <div class="table-container">
            <table>
                <thead>
                    <tr>
                        <th>Target</th>
                        <th>Type</th>
                        <th>Check</th>
                        <th>Status</th>
                        <th>Risk</th>
                        <th>Summary</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Inspections}}
                    <tr>
                        <td>{{.TargetName}}</td>
                        <td>{{.TargetType}}</td>
                        <td>{{.CheckType}}</td>
                        <td><span class="status-{{.Status}}">{{.Status}}</span></td>
                        <td>
                            <div class="risk-bar">
                                <div class="risk-fill" style="width:{{.RiskScore}}%; background:{{if le .RiskScore 30}}#27ae60{{else if le .RiskScore 60}}#f39c12{{else}}#e74c3c{{end}};"></div>
                            </div>
                            {{.RiskScore}}
                        </td>
                        <td>{{.Summary}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    </div>
</body>
</html>`
