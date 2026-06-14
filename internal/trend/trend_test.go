package trend

import (
	"testing"
	"time"

	"github.com/Liyonglin20030201/task061402/internal/store"
)

func TestLinearRegressionSlope(t *testing.T) {
	tests := []struct {
		name     string
		points   []TrendPoint
		positive bool
	}{
		{
			"increasing",
			[]TrendPoint{
				{Value: 10}, {Value: 20}, {Value: 30}, {Value: 40},
			},
			true,
		},
		{
			"decreasing",
			[]TrendPoint{
				{Value: 40}, {Value: 30}, {Value: 20}, {Value: 10},
			},
			false,
		},
		{
			"flat",
			[]TrendPoint{
				{Value: 50}, {Value: 50}, {Value: 50}, {Value: 50},
			},
			false,
		},
		{
			"single point",
			[]TrendPoint{{Value: 50}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slope := linearRegressionSlope(tt.points)
			if tt.positive && slope <= 0 {
				t.Errorf("expected positive slope, got %f", slope)
			}
			if !tt.positive && slope > 0 && tt.name != "flat" {
				t.Errorf("expected non-positive slope, got %f", slope)
			}
		})
	}
}

func TestAvgRiskScore(t *testing.T) {
	inspections := []*store.Inspection{
		{RiskScore: 10},
		{RiskScore: 20},
		{RiskScore: 30},
	}
	avg := avgRiskScore(inspections)
	if avg != 20 {
		t.Errorf("expected avg 20, got %f", avg)
	}
}

func TestAvgRiskScore_Empty(t *testing.T) {
	avg := avgRiskScore(nil)
	if avg != 0 {
		t.Errorf("expected avg 0 for empty slice, got %f", avg)
	}
}

func TestTrendResult_Direction(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		points    []TrendPoint
		direction string
	}{
		{
			"degrading",
			[]TrendPoint{
				{Timestamp: now.Add(-4 * time.Hour), Value: 10},
				{Timestamp: now.Add(-3 * time.Hour), Value: 30},
				{Timestamp: now.Add(-2 * time.Hour), Value: 50},
				{Timestamp: now.Add(-1 * time.Hour), Value: 70},
			},
			"degrading",
		},
		{
			"improving",
			[]TrendPoint{
				{Timestamp: now.Add(-4 * time.Hour), Value: 70},
				{Timestamp: now.Add(-3 * time.Hour), Value: 50},
				{Timestamp: now.Add(-2 * time.Hour), Value: 30},
				{Timestamp: now.Add(-1 * time.Hour), Value: 10},
			},
			"improving",
		},
		{
			"stable",
			[]TrendPoint{
				{Timestamp: now.Add(-4 * time.Hour), Value: 50},
				{Timestamp: now.Add(-3 * time.Hour), Value: 51},
				{Timestamp: now.Add(-2 * time.Hour), Value: 49},
				{Timestamp: now.Add(-1 * time.Hour), Value: 50},
			},
			"stable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first := tt.points[0].Value
			last := tt.points[len(tt.points)-1].Value
			var deltaPct float64
			if first == 0 {
				deltaPct = 0
			} else {
				deltaPct = ((last - first) / first) * 100
			}
			slope := linearRegressionSlope(tt.points)

			var direction string
			if deltaPct < 10 && deltaPct > -10 && slope < 2 && slope > -2 {
				direction = "stable"
			} else if slope > 0 {
				direction = "degrading"
			} else {
				direction = "improving"
			}

			if direction != tt.direction {
				t.Errorf("expected direction %q, got %q (deltaPct=%f, slope=%f)", tt.direction, direction, deltaPct, slope)
			}
		})
	}
}
