package trend

import (
	"fmt"
	"math"
	"time"

	"github.com/Liyonglin20030201/task061402/internal/store"
)

type TimeRange struct {
	Start time.Time
	End   time.Time
}

type TrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	RunID     string    `json:"run_id"`
}

type TrendResult struct {
	Metric     string       `json:"metric"`
	TargetName string       `json:"target_name"`
	Direction  string       `json:"direction"`
	DataPoints []TrendPoint `json:"data_points"`
	DeltaPct   float64      `json:"delta_pct"`
}

type ComparisonResult struct {
	TargetName string  `json:"target_name"`
	CheckType  string  `json:"check_type"`
	BeforeAvg  float64 `json:"before_avg"`
	AfterAvg   float64 `json:"after_avg"`
	ScoreDelta float64 `json:"score_delta"`
	Summary    string  `json:"summary"`
}

type Analyzer struct {
	store *store.Store
}

func NewAnalyzer(s *store.Store) *Analyzer {
	return &Analyzer{store: s}
}

func (a *Analyzer) DetectTrend(targetName, checkType string, tr TimeRange, limit int) (*TrendResult, error) {
	if limit <= 0 {
		limit = 100
	}

	inspections, err := a.store.GetInspectionsByCheckTypeAndTarget(targetName, checkType, tr.Start, tr.End, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query trend data: %w", err)
	}

	if len(inspections) < 2 {
		return &TrendResult{
			Metric:     checkType,
			TargetName: targetName,
			Direction:  "insufficient_data",
			DeltaPct:   0,
		}, nil
	}

	result := &TrendResult{
		Metric:     checkType,
		TargetName: targetName,
	}

	for i := len(inspections) - 1; i >= 0; i-- {
		insp := inspections[i]
		result.DataPoints = append(result.DataPoints, TrendPoint{
			Timestamp: insp.CreatedAt,
			Value:     float64(insp.RiskScore),
			RunID:     insp.RunID,
		})
	}

	first := result.DataPoints[0].Value
	last := result.DataPoints[len(result.DataPoints)-1].Value

	if first == 0 {
		if last == 0 {
			result.DeltaPct = 0
		} else {
			result.DeltaPct = 100
		}
	} else {
		result.DeltaPct = ((last - first) / first) * 100
	}

	slope := linearRegressionSlope(result.DataPoints)

	if math.Abs(result.DeltaPct) < 10 && math.Abs(slope) < 2 {
		result.Direction = "stable"
	} else if slope > 0 {
		result.Direction = "degrading"
	} else {
		result.Direction = "improving"
	}

	return result, nil
}

func (a *Analyzer) ComparePeriods(targetName string, before, after TimeRange) ([]*ComparisonResult, error) {
	checkTypes, err := a.store.GetCheckTypesForTarget(targetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get check types: %w", err)
	}

	var results []*ComparisonResult

	for _, ct := range checkTypes {
		beforeInsps, err := a.store.GetInspectionsByCheckTypeAndTarget(targetName, ct, before.Start, before.End, 100)
		if err != nil {
			continue
		}
		afterInsps, err := a.store.GetInspectionsByCheckTypeAndTarget(targetName, ct, after.Start, after.End, 100)
		if err != nil {
			continue
		}

		if len(beforeInsps) == 0 || len(afterInsps) == 0 {
			continue
		}

		beforeAvg := avgRiskScore(beforeInsps)
		afterAvg := avgRiskScore(afterInsps)
		delta := afterAvg - beforeAvg

		var summary string
		if delta > 5 {
			summary = fmt.Sprintf("%s risk increased by %.1f points", ct, delta)
		} else if delta < -5 {
			summary = fmt.Sprintf("%s risk decreased by %.1f points", ct, -delta)
		} else {
			summary = fmt.Sprintf("%s risk stable (%.1f → %.1f)", ct, beforeAvg, afterAvg)
		}

		results = append(results, &ComparisonResult{
			TargetName: targetName,
			CheckType:  ct,
			BeforeAvg:  beforeAvg,
			AfterAvg:   afterAvg,
			ScoreDelta: delta,
			Summary:    summary,
		})
	}

	return results, nil
}

func linearRegressionSlope(points []TrendPoint) float64 {
	n := float64(len(points))
	if n < 2 {
		return 0
	}

	var sumX, sumY, sumXY, sumX2 float64
	for i, p := range points {
		x := float64(i)
		y := p.Value
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0
	}
	return (n*sumXY - sumX*sumY) / denom
}

func avgRiskScore(inspections []*store.Inspection) float64 {
	if len(inspections) == 0 {
		return 0
	}
	var sum float64
	for _, insp := range inspections {
		sum += float64(insp.RiskScore)
	}
	return sum / float64(len(inspections))
}
