package notifier

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type AlertEvent struct {
	TargetName string                 `json:"target_name"`
	CheckType  string                 `json:"check_type"`
	RiskScore  int                    `json:"risk_score"`
	RiskLevel  string                 `json:"risk_level"`
	Summary    string                 `json:"summary"`
	Details    map[string]interface{} `json:"details,omitempty"`
	OccurredAt time.Time             `json:"occurred_at"`
	RunID      string                 `json:"run_id"`
}

type Channel interface {
	Name() string
	Send(ctx context.Context, event *AlertEvent) error
}

type ThresholdConfig struct {
	GlobalRiskScore    int
	CategoryThresholds map[string]int
}

type Dispatcher struct {
	channels   []Channel
	thresholds ThresholdConfig
}

func NewDispatcher(channels []Channel, thresholds ThresholdConfig) *Dispatcher {
	return &Dispatcher{
		channels:   channels,
		thresholds: thresholds,
	}
}

func (d *Dispatcher) Evaluate(ctx context.Context, overallScore int, categoryScores map[string]int, runID string) []error {
	var events []*AlertEvent

	if overallScore >= d.thresholds.GlobalRiskScore {
		events = append(events, &AlertEvent{
			TargetName: "overall",
			CheckType:  "risk_score",
			RiskScore:  overallScore,
			RiskLevel:  riskLevel(overallScore),
			Summary:    fmt.Sprintf("overall risk score %d exceeds threshold %d", overallScore, d.thresholds.GlobalRiskScore),
			OccurredAt: time.Now(),
			RunID:      runID,
		})
	}

	for category, score := range categoryScores {
		threshold, ok := d.thresholds.CategoryThresholds[category]
		if !ok {
			continue
		}
		if score >= threshold {
			events = append(events, &AlertEvent{
				TargetName: "category",
				CheckType:  category,
				RiskScore:  score,
				RiskLevel:  riskLevel(score),
				Summary:    fmt.Sprintf("%s risk score %d exceeds threshold %d", category, score, threshold),
				OccurredAt: time.Now(),
				RunID:      runID,
			})
		}
	}

	if len(events) == 0 {
		return nil
	}

	var mu sync.Mutex
	var errs []error
	var wg sync.WaitGroup

	for _, event := range events {
		for _, ch := range d.channels {
			wg.Add(1)
			go func(c Channel, e *AlertEvent) {
				defer wg.Done()
				if err := c.Send(ctx, e); err != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf("channel %s: %w", c.Name(), err))
					mu.Unlock()
				}
			}(ch, event)
		}
	}

	wg.Wait()
	return errs
}

func riskLevel(score int) string {
	switch {
	case score >= 80:
		return "CRITICAL"
	case score >= 60:
		return "HIGH"
	case score >= 40:
		return "MEDIUM"
	case score >= 20:
		return "LOW"
	default:
		return "HEALTHY"
	}
}
