package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Liyonglin20030201/task061402/internal/config"
	"github.com/Liyonglin20030201/task061402/internal/connector"
	"github.com/Liyonglin20030201/task061402/internal/inspector"
)

type Executor struct {
	pluginDir string
}

func NewExecutor(pluginDir string) *Executor {
	return &Executor{pluginDir: pluginDir}
}

func (e *Executor) Run(ctx context.Context, info PluginInfo, conn connector.Connector, cfg *config.Config) (*inspector.Result, error) {
	result := inspector.NewResult("plugin:" + info.Name)

	var target config.Target
	for _, t := range cfg.Targets {
		if t.Name == conn.Name() {
			target = t
			break
		}
	}

	envVars := []string{
		fmt.Sprintf("DBINSPECT_TARGET_NAME=%s", target.Name),
		fmt.Sprintf("DBINSPECT_TARGET_TYPE=%s", target.Type),
		fmt.Sprintf("DBINSPECT_TARGET_HOST=%s", target.Host),
		fmt.Sprintf("DBINSPECT_TARGET_PORT=%d", target.Port),
		fmt.Sprintf("DBINSPECT_TARGET_USER=%s", target.User),
		fmt.Sprintf("DBINSPECT_TARGET_DB=%s", target.Database),
	}

	var cmd *exec.Cmd
	switch {
	case strings.HasSuffix(info.Path, ".py"):
		cmd = exec.CommandContext(ctx, "python", info.Path)
	case strings.HasSuffix(info.Path, ".sh") || strings.HasSuffix(info.Path, ".bash"):
		cmd = exec.CommandContext(ctx, "bash", info.Path)
	default:
		cmd = exec.CommandContext(ctx, info.Path)
	}

	cmd.Env = append(cmd.Environ(), envVars...)

	start := time.Now()
	output, err := cmd.Output()
	duration := time.Since(start)

	result.Details["duration_ms"] = duration.Milliseconds()
	result.Details["plugin"] = info.Name

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.RiskScore = 50
			return result.Finish(inspector.StatusError, fmt.Sprintf("plugin %s timed out", info.Name)), nil
		}
		result.RiskScore = 30
		return result.Finish(inspector.StatusError, fmt.Sprintf("plugin %s failed: %v", info.Name, err)), nil
	}

	var pluginResult struct {
		Status    string                 `json:"status"`
		RiskScore int                    `json:"risk_score"`
		Summary   string                 `json:"summary"`
		Details   map[string]interface{} `json:"details"`
	}

	if err := json.Unmarshal(output, &pluginResult); err != nil {
		result.Details["raw_output"] = string(output)
		result.RiskScore = 0
		return result.Finish(inspector.StatusSuccess, fmt.Sprintf("plugin %s completed (non-JSON output)", info.Name)), nil
	}

	if pluginResult.Details != nil {
		for k, v := range pluginResult.Details {
			result.Details[k] = v
		}
	}
	result.RiskScore = pluginResult.RiskScore

	status := inspector.StatusSuccess
	switch pluginResult.Status {
	case "warning":
		status = inspector.StatusWarning
	case "error":
		status = inspector.StatusError
	}

	return result.Finish(status, pluginResult.Summary), nil
}
