package inspector

import (
	"context"
	"fmt"
	"time"

	"github.com/Liyonglin20030201/task061402/internal/config"
	"github.com/Liyonglin20030201/task061402/internal/connector"
)

type PingInspector struct{}

func NewPingInspector() *PingInspector { return &PingInspector{} }

func (p *PingInspector) Name() string { return "ping" }

func (p *PingInspector) Run(ctx context.Context, conn connector.Connector, cfg *config.Config) (*Result, error) {
	result := NewResult("ping")

	start := time.Now()
	err := conn.Ping(ctx)
	latency := time.Since(start)

	result.Details["latency_ms"] = latency.Milliseconds()
	result.Details["target"] = conn.Name()
	result.Details["type"] = conn.Type()

	if err != nil {
		result.RiskScore = 100
		return result.Finish(StatusError, fmt.Sprintf("connection failed: %v", err)), nil
	}

	if latency > 5*time.Second {
		result.RiskScore = 60
		return result.Finish(StatusWarning, fmt.Sprintf("connection slow: %dms", latency.Milliseconds())), nil
	}

	if latency > 1*time.Second {
		result.RiskScore = 30
		return result.Finish(StatusWarning, fmt.Sprintf("connection latency elevated: %dms", latency.Milliseconds())), nil
	}

	result.RiskScore = 0
	return result.Finish(StatusSuccess, fmt.Sprintf("connected successfully in %dms", latency.Milliseconds())), nil
}
