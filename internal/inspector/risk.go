package inspector

import (
	"context"
	"fmt"

	"github.com/Liyonglin20030201/task061402/internal/config"
	"github.com/Liyonglin20030201/task061402/internal/connector"
)

type RiskInspector struct{}

func NewRiskInspector() *RiskInspector { return &RiskInspector{} }

func (r *RiskInspector) Name() string { return "risk" }

func (r *RiskInspector) Run(ctx context.Context, conn connector.Connector, cfg *config.Config) (*Result, error) {
	result := NewResult("risk")
	return result.Finish(StatusSuccess, "risk scoring is computed from aggregated results"), nil
}

func ComputeRiskScore(results []*Result, weights map[string]int) (int, map[string]int) {
	if len(weights) == 0 {
		weights = map[string]int{
			"connection":  15,
			"slowquery":   15,
			"capacity":    12,
			"index":       12,
			"backup":      12,
			"permission":  12,
			"replication": 12,
			"schema":      10,
		}
	}

	checkTypeMap := map[string]string{
		"ping":        "connection",
		"slowquery":   "slowquery",
		"capacity":    "capacity",
		"index":       "index",
		"backup":      "backup",
		"permission":  "permission",
		"replication": "replication",
		"schema":      "schema",
	}

	scores := make(map[string]int)
	for _, r := range results {
		category, ok := checkTypeMap[r.CheckType]
		if !ok {
			continue
		}
		if existing, exists := scores[category]; exists {
			if r.RiskScore > existing {
				scores[category] = r.RiskScore
			}
		} else {
			scores[category] = r.RiskScore
		}
	}

	totalWeight := 0
	weightedScore := 0
	for category, weight := range weights {
		score, ok := scores[category]
		if !ok {
			continue
		}
		totalWeight += weight
		weightedScore += score * weight
	}

	var finalScore int
	if totalWeight > 0 {
		finalScore = weightedScore / totalWeight
	}

	return finalScore, scores
}

func RiskLevel(score int) string {
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

func RiskSummary(score int) string {
	return fmt.Sprintf("Overall Risk Score: %d/100 (%s)", score, RiskLevel(score))
}
