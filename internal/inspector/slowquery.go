package inspector

import (
	"context"
	"fmt"

	"github.com/Liyonglin20030201/task061402/internal/config"
	"github.com/Liyonglin20030201/task061402/internal/connector"
)

type SlowQueryInspector struct{}

func NewSlowQueryInspector() *SlowQueryInspector { return &SlowQueryInspector{} }

func (s *SlowQueryInspector) Name() string { return "slowquery" }

func (s *SlowQueryInspector) Run(ctx context.Context, conn connector.Connector, cfg *config.Config) (*Result, error) {
	result := NewResult("slowquery")

	if !cfg.Checks.SlowQuery.Enabled {
		return result.Finish(StatusSkipped, "slow query check disabled"), nil
	}

	switch conn.Type() {
	case "mysql":
		return s.runMySQL(ctx, conn, cfg, result)
	case "postgres":
		return s.runPostgres(ctx, conn, cfg, result)
	case "redis":
		return s.runRedis(ctx, conn, cfg, result)
	default:
		return result.Finish(StatusSkipped, fmt.Sprintf("unsupported type: %s", conn.Type())), nil
	}
}

func (s *SlowQueryInspector) runMySQL(ctx context.Context, conn connector.Connector, cfg *config.Config, result *Result) (*Result, error) {
	sqlConn, ok := conn.(connector.SQLConnector)
	if !ok {
		return result.Finish(StatusError, "invalid connector type for MySQL"), nil
	}

	thresholdSec := cfg.Checks.SlowQuery.Threshold.Seconds()
	query := fmt.Sprintf(`
		SELECT digest_text, count_star, avg_timer_wait/1000000000000 as avg_sec
		FROM performance_schema.events_statements_summary_by_digest
		WHERE avg_timer_wait/1000000000000 > %f
		ORDER BY avg_timer_wait DESC
		LIMIT %d`, thresholdSec, cfg.Checks.SlowQuery.TopN)

	rows, err := sqlConn.Query(ctx, query)
	if err != nil {
		result.RiskScore = 30
		return result.Finish(StatusWarning, fmt.Sprintf("failed to query slow log (permission issue?): %v", err)), nil
	}
	defer rows.Close()

	var slowQueries []map[string]interface{}
	for rows.Next() {
		var digestText string
		var countStar int64
		var avgSec float64
		if err := rows.Scan(&digestText, &countStar, &avgSec); err != nil {
			continue
		}
		slowQueries = append(slowQueries, map[string]interface{}{
			"query":     digestText,
			"count":     countStar,
			"avg_sec":   avgSec,
		})
	}

	result.Details["slow_queries"] = slowQueries
	result.Details["count"] = len(slowQueries)
	result.Details["threshold_sec"] = thresholdSec

	if len(slowQueries) == 0 {
		result.RiskScore = 0
		return result.Finish(StatusSuccess, "no slow queries detected"), nil
	}

	if len(slowQueries) > 10 {
		result.RiskScore = 80
		return result.Finish(StatusWarning, fmt.Sprintf("%d slow queries detected (high risk)", len(slowQueries))), nil
	}

	result.RiskScore = 40
	return result.Finish(StatusWarning, fmt.Sprintf("%d slow queries detected", len(slowQueries))), nil
}

func (s *SlowQueryInspector) runPostgres(ctx context.Context, conn connector.Connector, cfg *config.Config, result *Result) (*Result, error) {
	sqlConn, ok := conn.(connector.SQLConnector)
	if !ok {
		return result.Finish(StatusError, "invalid connector type for PostgreSQL"), nil
	}

	thresholdMs := cfg.Checks.SlowQuery.Threshold.Milliseconds()
	query := fmt.Sprintf(`
		SELECT query, calls, mean_exec_time
		FROM pg_stat_statements
		WHERE mean_exec_time > %d
		ORDER BY mean_exec_time DESC
		LIMIT %d`, thresholdMs, cfg.Checks.SlowQuery.TopN)

	rows, err := sqlConn.Query(ctx, query)
	if err != nil {
		result.RiskScore = 30
		return result.Finish(StatusWarning, fmt.Sprintf("failed to query pg_stat_statements (extension not enabled?): %v", err)), nil
	}
	defer rows.Close()

	var slowQueries []map[string]interface{}
	for rows.Next() {
		var queryText string
		var calls int64
		var meanTime float64
		if err := rows.Scan(&queryText, &calls, &meanTime); err != nil {
			continue
		}
		slowQueries = append(slowQueries, map[string]interface{}{
			"query":      queryText,
			"calls":      calls,
			"mean_ms":    meanTime,
		})
	}

	result.Details["slow_queries"] = slowQueries
	result.Details["count"] = len(slowQueries)

	if len(slowQueries) == 0 {
		result.RiskScore = 0
		return result.Finish(StatusSuccess, "no slow queries detected"), nil
	}

	result.RiskScore = 50
	return result.Finish(StatusWarning, fmt.Sprintf("%d slow queries detected", len(slowQueries))), nil
}

func (s *SlowQueryInspector) runRedis(ctx context.Context, conn connector.Connector, cfg *config.Config, result *Result) (*Result, error) {
	redisConn, ok := conn.(*connector.RedisConnector)
	if !ok {
		return result.Finish(StatusError, "invalid connector type for Redis"), nil
	}

	entries, err := redisConn.SlowLogGet(ctx, int64(cfg.Checks.SlowQuery.TopN))
	if err != nil {
		result.RiskScore = 30
		return result.Finish(StatusWarning, fmt.Sprintf("failed to get slowlog: %v", err)), nil
	}

	result.Details["slow_commands"] = entries
	result.Details["count"] = len(entries)

	if len(entries) == 0 {
		result.RiskScore = 0
		return result.Finish(StatusSuccess, "no slow commands in Redis slowlog"), nil
	}

	result.RiskScore = 40
	return result.Finish(StatusWarning, fmt.Sprintf("%d slow commands found in Redis slowlog", len(entries))), nil
}
