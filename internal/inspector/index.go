package inspector

import (
	"context"
	"fmt"

	"github.com/Liyonglin20030201/task061402/internal/config"
	"github.com/Liyonglin20030201/task061402/internal/connector"
)

type IndexInspector struct{}

func NewIndexInspector() *IndexInspector { return &IndexInspector{} }

func (i *IndexInspector) Name() string { return "index" }

func (i *IndexInspector) Run(ctx context.Context, conn connector.Connector, cfg *config.Config) (*Result, error) {
	result := NewResult("index")

	if !cfg.Checks.Index.Enabled {
		return result.Finish(StatusSkipped, "index check disabled"), nil
	}

	switch conn.Type() {
	case "mysql":
		return i.runMySQL(ctx, conn, cfg, result)
	case "postgres":
		return i.runPostgres(ctx, conn, cfg, result)
	case "redis":
		return result.Finish(StatusSkipped, "index check not applicable to Redis"), nil
	default:
		return result.Finish(StatusSkipped, fmt.Sprintf("unsupported type: %s", conn.Type())), nil
	}
}

func (i *IndexInspector) runMySQL(ctx context.Context, conn connector.Connector, cfg *config.Config, result *Result) (*Result, error) {
	sqlConn, ok := conn.(connector.SQLConnector)
	if !ok {
		return result.Finish(StatusError, "invalid connector type"), nil
	}

	// Duplicate indexes
	dupQuery := `
		SELECT t.TABLE_SCHEMA, t.TABLE_NAME,
			GROUP_CONCAT(t.INDEX_NAME) as duplicate_indexes,
			t.COLUMN_NAME
		FROM information_schema.STATISTICS t
		JOIN (
			SELECT TABLE_SCHEMA, TABLE_NAME, COLUMN_NAME, COUNT(*) as cnt
			FROM information_schema.STATISTICS
			WHERE SEQ_IN_INDEX = 1
			GROUP BY TABLE_SCHEMA, TABLE_NAME, COLUMN_NAME
			HAVING cnt > 1
		) dup ON t.TABLE_SCHEMA = dup.TABLE_SCHEMA
			AND t.TABLE_NAME = dup.TABLE_NAME
			AND t.COLUMN_NAME = dup.COLUMN_NAME
		WHERE t.SEQ_IN_INDEX = 1
		GROUP BY t.TABLE_SCHEMA, t.TABLE_NAME, t.COLUMN_NAME`

	rows, err := sqlConn.Query(ctx, dupQuery)
	if err != nil {
		result.Details["duplicate_error"] = err.Error()
	} else {
		defer rows.Close()
		var duplicates []map[string]interface{}
		for rows.Next() {
			var schema, table, indexes, column string
			if err := rows.Scan(&schema, &table, &indexes, &column); err != nil {
				continue
			}
			duplicates = append(duplicates, map[string]interface{}{
				"schema":  schema,
				"table":   table,
				"indexes": indexes,
				"column":  column,
			})
		}
		result.Details["duplicate_indexes"] = duplicates
	}

	// Tables without primary key
	noPKQuery := fmt.Sprintf(`
		SELECT t.TABLE_SCHEMA, t.TABLE_NAME, t.TABLE_ROWS
		FROM information_schema.TABLES t
		LEFT JOIN information_schema.TABLE_CONSTRAINTS tc
			ON t.TABLE_SCHEMA = tc.TABLE_SCHEMA
			AND t.TABLE_NAME = tc.TABLE_NAME
			AND tc.CONSTRAINT_TYPE = 'PRIMARY KEY'
		WHERE tc.CONSTRAINT_NAME IS NULL
			AND t.TABLE_TYPE = 'BASE TABLE'
			AND t.TABLE_ROWS > %d`, cfg.Checks.Index.MinTableRows)

	rows2, err := sqlConn.Query(ctx, noPKQuery)
	if err != nil {
		result.Details["no_pk_error"] = err.Error()
	} else {
		defer rows2.Close()
		var noPK []map[string]interface{}
		for rows2.Next() {
			var schema, table string
			var rowCount int64
			if err := rows2.Scan(&schema, &table, &rowCount); err != nil {
				continue
			}
			noPK = append(noPK, map[string]interface{}{
				"schema": schema,
				"table":  table,
				"rows":   rowCount,
			})
		}
		result.Details["tables_without_pk"] = noPK
	}

	issues := 0
	if dups, ok := result.Details["duplicate_indexes"].([]map[string]interface{}); ok {
		issues += len(dups)
	}
	if noPK, ok := result.Details["tables_without_pk"].([]map[string]interface{}); ok {
		issues += len(noPK)
	}

	if issues == 0 {
		result.RiskScore = 0
		return result.Finish(StatusSuccess, "no index issues found"), nil
	}

	result.RiskScore = min(issues*15, 80)
	return result.Finish(StatusWarning, fmt.Sprintf("%d index issues found", issues)), nil
}

func (i *IndexInspector) runPostgres(ctx context.Context, conn connector.Connector, cfg *config.Config, result *Result) (*Result, error) {
	sqlConn, ok := conn.(connector.SQLConnector)
	if !ok {
		return result.Finish(StatusError, "invalid connector type"), nil
	}

	// Unused indexes
	unusedQuery := `
		SELECT schemaname, relname, indexrelname, idx_scan
		FROM pg_stat_user_indexes
		WHERE idx_scan = 0
		ORDER BY pg_relation_size(indexrelid) DESC
		LIMIT 20`

	rows, err := sqlConn.Query(ctx, unusedQuery)
	if err != nil {
		result.Details["unused_error"] = err.Error()
	} else {
		defer rows.Close()
		var unused []map[string]interface{}
		for rows.Next() {
			var schema, table, indexName string
			var scans int64
			if err := rows.Scan(&schema, &table, &indexName, &scans); err != nil {
				continue
			}
			unused = append(unused, map[string]interface{}{
				"schema": schema,
				"table":  table,
				"index":  indexName,
				"scans":  scans,
			})
		}
		result.Details["unused_indexes"] = unused
	}

	// Duplicate indexes
	dupQuery := `
		SELECT pg_size_pretty(sum(pg_relation_size(idx))::bigint) as size,
			(array_agg(idx))[1] as idx1, (array_agg(idx))[2] as idx2,
			(array_agg(pg_get_indexdef(idx)))[1] as def
		FROM (
			SELECT indexrelid::regclass as idx,
				(indrelid::text || E'\n' || indclass::text || E'\n' || indkey::text) as key
			FROM pg_index
		) sub
		GROUP BY key HAVING count(*) > 1
		ORDER BY sum(pg_relation_size(idx)) DESC
		LIMIT 10`

	rows2, err := sqlConn.Query(ctx, dupQuery)
	if err != nil {
		result.Details["duplicate_error"] = err.Error()
	} else {
		defer rows2.Close()
		var duplicates []map[string]interface{}
		for rows2.Next() {
			var size, idx1, idx2, def string
			if err := rows2.Scan(&size, &idx1, &idx2, &def); err != nil {
				continue
			}
			duplicates = append(duplicates, map[string]interface{}{
				"size":       size,
				"index1":     idx1,
				"index2":     idx2,
				"definition": def,
			})
		}
		result.Details["duplicate_indexes"] = duplicates
	}

	issues := 0
	if unused, ok := result.Details["unused_indexes"].([]map[string]interface{}); ok {
		issues += len(unused)
	}
	if dups, ok := result.Details["duplicate_indexes"].([]map[string]interface{}); ok {
		issues += len(dups)
	}

	if issues == 0 {
		result.RiskScore = 0
		return result.Finish(StatusSuccess, "no index issues found"), nil
	}

	result.RiskScore = min(issues*10, 70)
	return result.Finish(StatusWarning, fmt.Sprintf("%d index issues found", issues)), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
