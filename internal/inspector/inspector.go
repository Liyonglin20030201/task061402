package inspector

import (
	"context"
	"time"

	"github.com/Liyonglin20030201/task061402/internal/config"
	"github.com/Liyonglin20030201/task061402/internal/connector"
)

type Status string

const (
	StatusSuccess Status = "success"
	StatusWarning Status = "warning"
	StatusError   Status = "error"
	StatusSkipped Status = "skipped"
)

type Result struct {
	CheckType  string
	Status     Status
	RiskScore  int
	Summary    string
	Details    map[string]interface{}
	StartedAt  time.Time
	FinishedAt time.Time
}

type Inspector interface {
	Name() string
	Run(ctx context.Context, conn connector.Connector, cfg *config.Config) (*Result, error)
}

func NewResult(checkType string) *Result {
	return &Result{
		CheckType: checkType,
		StartedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}
}

func (r *Result) Finish(status Status, summary string) *Result {
	r.Status = status
	r.Summary = summary
	r.FinishedAt = time.Now()
	return r
}
