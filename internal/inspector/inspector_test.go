package inspector

import (
	"testing"
)

func TestComputeRiskScore(t *testing.T) {
	results := []*Result{
		{CheckType: "ping", RiskScore: 0},
		{CheckType: "slowquery", RiskScore: 40},
		{CheckType: "capacity", RiskScore: 50},
		{CheckType: "index", RiskScore: 30},
		{CheckType: "backup", RiskScore: 100},
		{CheckType: "permission", RiskScore: 0},
	}

	weights := map[string]int{
		"connection": 20,
		"slowquery":  20,
		"capacity":   15,
		"index":      15,
		"backup":     15,
		"permission": 15,
	}

	score, categoryScores := ComputeRiskScore(results, weights)

	if score < 0 || score > 100 {
		t.Errorf("risk score out of range: %d", score)
	}

	if categoryScores["connection"] != 0 {
		t.Errorf("expected connection score 0, got %d", categoryScores["connection"])
	}

	if categoryScores["backup"] != 100 {
		t.Errorf("expected backup score 100, got %d", categoryScores["backup"])
	}
}

func TestRiskLevel(t *testing.T) {
	tests := []struct {
		score    int
		expected string
	}{
		{0, "HEALTHY"},
		{19, "HEALTHY"},
		{20, "LOW"},
		{39, "LOW"},
		{40, "MEDIUM"},
		{59, "MEDIUM"},
		{60, "HIGH"},
		{79, "HIGH"},
		{80, "CRITICAL"},
		{100, "CRITICAL"},
	}

	for _, tt := range tests {
		result := RiskLevel(tt.score)
		if result != tt.expected {
			t.Errorf("RiskLevel(%d) = %q, want %q", tt.score, result, tt.expected)
		}
	}
}

func TestNewResult(t *testing.T) {
	r := NewResult("ping")
	if r.CheckType != "ping" {
		t.Errorf("expected CheckType 'ping', got %q", r.CheckType)
	}
	if r.StartedAt.IsZero() {
		t.Error("expected StartedAt to be set")
	}
	if r.Details == nil {
		t.Error("expected Details to be initialized")
	}
}

func TestResultFinish(t *testing.T) {
	r := NewResult("capacity")
	r.Finish(StatusWarning, "high usage")

	if r.Status != StatusWarning {
		t.Errorf("expected status Warning, got %v", r.Status)
	}
	if r.Summary != "high usage" {
		t.Errorf("expected summary 'high usage', got %q", r.Summary)
	}
	if r.FinishedAt.IsZero() {
		t.Error("expected FinishedAt to be set")
	}
}
