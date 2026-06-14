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

	// MySQL performance_schema.events_statements_summary_by_digest:
	//   avg_timer_wait 单位为皮秒 (picoseconds, 10^-12 s)
	//   转换为秒: avg_timer_wait / 1000000000000
	thresholdSec := cfg.Checks.SlowQuery.Threshold.Seconds()
	topN := cfg.Checks.SlowQuery.TopN

	result.Details["threshold"] = cfg.Checks.SlowQuery.Threshold.String()
	result.Details["threshold_sec"] = thresholdSec
	result.Details["top_n"] = topN

	query := fmt.Sprintf(`
		SELECT COALESCE(digest_text, '(unknown)') AS digest_text,
		       count_star,
		       ROUND(avg_timer_wait / 1000000000000.0, 3) AS avg_sec
		FROM performance_schema.events_statements_summary_by_digest
		WHERE avg_timer_wait / 1000000000000.0 > %f
		  AND digest_text IS NOT NULL
		ORDER BY avg_timer_wait DESC
		LIMIT %d`, thresholdSec, topN)

	rows, err := sqlConn.Query(ctx, query)
	if err != nil {
		// 权限不足访问 performance_schema 时，降级为检查 slow_query_log 状态
		result.RiskScore = 30
		result.Details["error"] = err.Error()
		result.Details["fallback"] = "performance_schema access denied, trying SHOW GLOBAL STATUS"

		statusRow := sqlConn.QueryRow(ctx, "SHOW GLOBAL STATUS LIKE 'Slow_queries'")
		var varName, slowCount string
		if scanErr := statusRow.Scan(&varName, &slowCount); scanErr == nil {
			result.Details["slow_queries_total"] = slowCount
			return result.Finish(StatusWarning,
				fmt.Sprintf("cannot access performance_schema (%v), global slow_queries count: %s", err, slowCount)), nil
		}
		return result.Finish(StatusWarning,
			fmt.Sprintf("insufficient privilege to analyze slow queries: %v", err)), nil
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
			"query":   truncateQuery(digestText, 200),
			"count":   countStar,
			"avg_sec": avgSec,
		})
	}

	result.Details["slow_queries"] = slowQueries
	result.Details["count"] = len(slowQueries)

	if len(slowQueries) == 0 {
		result.RiskScore = 0
		return result.Finish(StatusSuccess,
			fmt.Sprintf("no slow queries above threshold %s", cfg.Checks.SlowQuery.Threshold)), nil
	}

	if len(slowQueries) > 10 {
		result.RiskScore = 80
		return result.Finish(StatusWarning,
			fmt.Sprintf("%d slow queries above %s (high risk)", len(slowQueries), cfg.Checks.SlowQuery.Threshold)), nil
	}

	result.RiskScore = 40
	return result.Finish(StatusWarning,
		fmt.Sprintf("%d slow queries above %s", len(slowQueries), cfg.Checks.SlowQuery.Threshold)), nil
}

func (s *SlowQueryInspector) runPostgres(ctx context.Context, conn connector.Connector, cfg *config.Config, result *Result) (*Result, error) {
	sqlConn, ok := conn.(connector.SQLConnector)
	if !ok {
		return result.Finish(StatusError, "invalid connector type for PostgreSQL"), nil
	}

	// PostgreSQL pg_stat_statements:
	//   mean_exec_time 单位为毫秒 (ms)
	//   配置阈值转换为毫秒进行比较
	thresholdMs := float64(cfg.Checks.SlowQuery.Threshold.Milliseconds())
	topN := cfg.Checks.SlowQuery.TopN

	result.Details["threshold"] = cfg.Checks.SlowQuery.Threshold.String()
	result.Details["threshold_ms"] = thresholdMs
	result.Details["top_n"] = topN

	query := fmt.Sprintf(`
		SELECT query, calls, ROUND(mean_exec_time::numeric, 2) AS mean_ms,
		       ROUND(total_exec_time::numeric, 2) AS total_ms
		FROM pg_stat_statements
		WHERE mean_exec_time > %f
		ORDER BY mean_exec_time DESC
		LIMIT %d`, thresholdMs, topN)

	rows, err := sqlConn.Query(ctx, query)
	if err != nil {
		// pg_stat_statements 扩展可能未安装，或权限不足
		result.RiskScore = 20
		result.Details["error"] = err.Error()

		// 降级检查: 尝试 pg_stat_activity 中当前长查询
		fallbackQuery := fmt.Sprintf(`
			SELECT query, state, EXTRACT(EPOCH FROM (now() - query_start)) AS running_sec
			FROM pg_stat_activity
			WHERE state = 'active' AND query_start < now() - interval '%d milliseconds'
			LIMIT %d`, int64(thresholdMs), topN)

		fallbackRows, fallbackErr := sqlConn.Query(ctx, fallbackQuery)
		if fallbackErr == nil {
			defer fallbackRows.Close()
			var activeSlows []map[string]interface{}
			for fallbackRows.Next() {
				var q, state string
				var runningSec float64
				if scanErr := fallbackRows.Scan(&q, &state, &runningSec); scanErr == nil {
					activeSlows = append(activeSlows, map[string]interface{}{
						"query":       truncateQuery(q, 200),
						"state":       state,
						"running_sec": runningSec,
					})
				}
			}
			if len(activeSlows) > 0 {
				result.Details["active_slow_queries"] = activeSlows
				return result.Finish(StatusWarning,
					fmt.Sprintf("pg_stat_statements unavailable, found %d active slow queries via pg_stat_activity", len(activeSlows))), nil
			}
		}

		return result.Finish(StatusWarning,
			fmt.Sprintf("pg_stat_statements not available: %v (extension may need to be enabled)", err)), nil
	}
	defer rows.Close()

	var slowQueries []map[string]interface{}
	for rows.Next() {
		var queryText string
		var calls int64
		var meanMs, totalMs float64
		if err := rows.Scan(&queryText, &calls, &meanMs, &totalMs); err != nil {
			continue
		}
		slowQueries = append(slowQueries, map[string]interface{}{
			"query":    truncateQuery(queryText, 200),
			"calls":    calls,
			"mean_ms":  meanMs,
			"total_ms": totalMs,
		})
	}

	result.Details["slow_queries"] = slowQueries
	result.Details["count"] = len(slowQueries)

	if len(slowQueries) == 0 {
		result.RiskScore = 0
		return result.Finish(StatusSuccess,
			fmt.Sprintf("no slow queries above threshold %s", cfg.Checks.SlowQuery.Threshold)), nil
	}

	if len(slowQueries) > 10 {
		result.RiskScore = 70
		return result.Finish(StatusWarning,
			fmt.Sprintf("%d slow queries above %s (high risk)", len(slowQueries), cfg.Checks.SlowQuery.Threshold)), nil
	}

	result.RiskScore = 50
	return result.Finish(StatusWarning,
		fmt.Sprintf("%d slow queries above %s", len(slowQueries), cfg.Checks.SlowQuery.Threshold)), nil
}

func (s *SlowQueryInspector) runRedis(ctx context.Context, conn connector.Connector, cfg *config.Config, result *Result) (*Result, error) {
	redisConn, ok := conn.(*connector.RedisConnector)
	if !ok {
		return result.Finish(StatusError, "invalid connector type for Redis"), nil
	}

	// Redis SLOWLOG:
	//   duration 单位为微秒 (microseconds, 10^-6 s)
	//   配置阈值转换为微秒进行比较
	thresholdUs := cfg.Checks.SlowQuery.Threshold.Microseconds()
	topN := cfg.Checks.SlowQuery.TopN

	result.Details["threshold"] = cfg.Checks.SlowQuery.Threshold.String()
	result.Details["threshold_us"] = thresholdUs
	result.Details["top_n"] = topN

	entries, err := redisConn.SlowLogGet(ctx, int64(topN))
	if err != nil {
		result.RiskScore = 30
		result.Details["error"] = err.Error()
		return result.Finish(StatusWarning, fmt.Sprintf("failed to get slowlog: %v", err)), nil
	}

	// 按配置阈值过滤（Redis slowlog-log-slower-than 是服务端配置，
	// 客户端再按我们的阈值做二次过滤）
	var filtered []map[string]interface{}
	for _, entry := range entries {
		if entry.Duration >= thresholdUs {
			filtered = append(filtered, map[string]interface{}{
				"id":          entry.ID,
				"timestamp":   entry.Timestamp,
				"duration_us": entry.Duration,
				"client":      entry.ClientAddr,
			})
		}
	}

	result.Details["slow_commands"] = filtered
	result.Details["total_slowlog_entries"] = len(entries)
	result.Details["count"] = len(filtered)

	if len(filtered) == 0 {
		result.RiskScore = 0
		return result.Finish(StatusSuccess,
			fmt.Sprintf("no slow commands above threshold %s in Redis slowlog (%d entries checked)",
				cfg.Checks.SlowQuery.Threshold, len(entries))), nil
	}

	result.RiskScore = 40
	return result.Finish(StatusWarning,
		fmt.Sprintf("%d slow commands above %s in Redis slowlog", len(filtered), cfg.Checks.SlowQuery.Threshold)), nil
}

func truncateQuery(q string, maxLen int) string {
	if len(q) <= maxLen {
		return q
	}
	return q[:maxLen] + "..."
}
