package inspector

import (
	"context"
	"fmt"
	"strings"

	"github.com/Liyonglin20030201/task061402/internal/config"
	"github.com/Liyonglin20030201/task061402/internal/connector"
)

type CapacityInspector struct{}

func NewCapacityInspector() *CapacityInspector { return &CapacityInspector{} }

func (c *CapacityInspector) Name() string { return "capacity" }

func (c *CapacityInspector) Run(ctx context.Context, conn connector.Connector, cfg *config.Config) (*Result, error) {
	result := NewResult("capacity")

	if !cfg.Checks.Capacity.Enabled {
		return result.Finish(StatusSkipped, "capacity check disabled"), nil
	}

	// 为大库扫描创建独立的超时 context，防止千万级表查询卡死
	scanCtx, scanCancel := context.WithTimeout(ctx, cfg.Checks.Capacity.ScanTimeout)
	defer scanCancel()

	switch conn.Type() {
	case "mysql":
		return c.runMySQL(scanCtx, conn, cfg, result)
	case "postgres":
		return c.runPostgres(scanCtx, conn, cfg, result)
	case "redis":
		return c.runRedis(scanCtx, conn, cfg, result)
	default:
		return result.Finish(StatusSkipped, fmt.Sprintf("unsupported type: %s", conn.Type())), nil
	}
}

func (c *CapacityInspector) runMySQL(ctx context.Context, conn connector.Connector, cfg *config.Config, result *Result) (*Result, error) {
	sqlConn, ok := conn.(connector.SQLConnector)
	if !ok {
		return result.Finish(StatusError, "invalid connector type"), nil
	}

	query := `SELECT table_schema,
		ROUND(SUM(data_length + index_length) / 1024 / 1024 / 1024, 2) as size_gb,
		SUM(table_rows) as total_rows
		FROM information_schema.tables
		GROUP BY table_schema
		ORDER BY size_gb DESC`

	rows, err := sqlConn.Query(ctx, query)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.RiskScore = 60
			result.Details["timeout"] = true
			result.Details["scan_timeout"] = cfg.Checks.Capacity.ScanTimeout.String()
			log := fmt.Sprintf("capacity scan timed out after %s (scan_timeout exceeded), skipping", cfg.Checks.Capacity.ScanTimeout)
			return result.Finish(StatusWarning, log), nil
		}
		return result.Finish(StatusError, fmt.Sprintf("failed to query capacity: %v", err)), nil
	}
	defer rows.Close()

	var databases []map[string]interface{}
	var totalGB float64
	for rows.Next() {
		// 在迭代过程中也检查上下文超时
		if ctx.Err() != nil {
			result.Details["partial"] = true
			result.Details["timeout"] = true
			result.Details["databases"] = databases
			result.Details["total_size_gb"] = totalGB
			result.RiskScore = 50
			return result.Finish(StatusWarning,
				fmt.Sprintf("capacity scan timed out after partial read (%.2f GB scanned so far), increase checks.capacity.scan_timeout", totalGB)), nil
		}

		var schema string
		var sizeGB float64
		var totalRows int64
		if err := rows.Scan(&schema, &sizeGB, &totalRows); err != nil {
			continue
		}
		totalGB += sizeGB
		databases = append(databases, map[string]interface{}{
			"schema":     schema,
			"size_gb":    sizeGB,
			"total_rows": totalRows,
		})
	}

	result.Details["databases"] = databases
	result.Details["total_size_gb"] = totalGB

	return c.evaluate(result, totalGB, cfg)
}

func (c *CapacityInspector) runPostgres(ctx context.Context, conn connector.Connector, cfg *config.Config, result *Result) (*Result, error) {
	sqlConn, ok := conn.(connector.SQLConnector)
	if !ok {
		return result.Finish(StatusError, "invalid connector type"), nil
	}

	query := `SELECT pg_database.datname,
		pg_database_size(pg_database.datname) / 1024.0 / 1024.0 / 1024.0 as size_gb
		FROM pg_database
		WHERE datistemplate = false
		ORDER BY size_gb DESC`

	rows, err := sqlConn.Query(ctx, query)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.RiskScore = 60
			result.Details["timeout"] = true
			result.Details["scan_timeout"] = cfg.Checks.Capacity.ScanTimeout.String()
			return result.Finish(StatusWarning,
				fmt.Sprintf("capacity scan timed out after %s, skipping", cfg.Checks.Capacity.ScanTimeout)), nil
		}
		return result.Finish(StatusError, fmt.Sprintf("failed to query capacity: %v", err)), nil
	}
	defer rows.Close()

	var databases []map[string]interface{}
	var totalGB float64
	for rows.Next() {
		if ctx.Err() != nil {
			result.Details["partial"] = true
			result.Details["timeout"] = true
			result.Details["databases"] = databases
			result.Details["total_size_gb"] = totalGB
			result.RiskScore = 50
			return result.Finish(StatusWarning,
				fmt.Sprintf("capacity scan timed out after partial read (%.2f GB scanned so far)", totalGB)), nil
		}

		var dbName string
		var sizeGB float64
		if err := rows.Scan(&dbName, &sizeGB); err != nil {
			continue
		}
		totalGB += sizeGB
		databases = append(databases, map[string]interface{}{
			"database": dbName,
			"size_gb":  sizeGB,
		})
	}

	result.Details["databases"] = databases
	result.Details["total_size_gb"] = totalGB

	return c.evaluate(result, totalGB, cfg)
}

func (c *CapacityInspector) runRedis(ctx context.Context, conn connector.Connector, cfg *config.Config, result *Result) (*Result, error) {
	redisConn, ok := conn.(*connector.RedisConnector)
	if !ok {
		return result.Finish(StatusError, "invalid connector type"), nil
	}

	info, err := redisConn.Info(ctx, "memory")
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.RiskScore = 40
			result.Details["timeout"] = true
			return result.Finish(StatusWarning, "Redis info command timed out"), nil
		}
		return result.Finish(StatusError, fmt.Sprintf("failed to get Redis info: %v", err)), nil
	}

	var usedMemoryBytes int64
	var maxMemoryBytes int64
	for _, line := range strings.Split(info, "\r\n") {
		if strings.HasPrefix(line, "used_memory:") {
			fmt.Sscanf(strings.TrimPrefix(line, "used_memory:"), "%d", &usedMemoryBytes)
		}
		if strings.HasPrefix(line, "maxmemory:") {
			fmt.Sscanf(strings.TrimPrefix(line, "maxmemory:"), "%d", &maxMemoryBytes)
		}
	}

	usedGB := float64(usedMemoryBytes) / 1024.0 / 1024.0 / 1024.0
	result.Details["used_memory_bytes"] = usedMemoryBytes
	result.Details["used_memory_gb"] = usedGB
	result.Details["max_memory_bytes"] = maxMemoryBytes

	if maxMemoryBytes > 0 {
		usagePercent := float64(usedMemoryBytes) / float64(maxMemoryBytes) * 100
		result.Details["usage_percent"] = usagePercent
		if usagePercent > 90 {
			result.RiskScore = 80
			return result.Finish(StatusWarning, fmt.Sprintf("Redis memory usage at %.1f%%", usagePercent)), nil
		}
	}

	return c.evaluate(result, usedGB, cfg)
}

func (c *CapacityInspector) evaluate(result *Result, totalGB float64, cfg *config.Config) (*Result, error) {
	if cfg.Checks.Capacity.CriticalThresholdGB > 0 && totalGB >= cfg.Checks.Capacity.CriticalThresholdGB {
		result.RiskScore = 90
		return result.Finish(StatusError, fmt.Sprintf("capacity critical: %.2f GB exceeds threshold", totalGB)), nil
	}

	if cfg.Checks.Capacity.WarnThresholdGB > 0 && totalGB >= cfg.Checks.Capacity.WarnThresholdGB {
		result.RiskScore = 50
		return result.Finish(StatusWarning, fmt.Sprintf("capacity warning: %.2f GB", totalGB)), nil
	}

	result.RiskScore = 0
	return result.Finish(StatusSuccess, fmt.Sprintf("capacity normal: %.2f GB", totalGB)), nil
}
