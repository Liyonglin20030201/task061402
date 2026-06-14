package inspector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Liyonglin20030201/task061402/internal/config"
	"github.com/Liyonglin20030201/task061402/internal/connector"
	"github.com/Liyonglin20030201/task061402/internal/store"
)

type SchemaSnapshot struct {
	Tables []TableSchema `json:"tables"`
}

type TableSchema struct {
	Name    string         `json:"name"`
	Columns []ColumnSchema `json:"columns"`
	Indexes []IndexSchema  `json:"indexes"`
}

type ColumnSchema struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Default  string `json:"default"`
}

type IndexSchema struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
	Primary bool     `json:"primary"`
}

type SchemaDiff struct {
	AddedTables     []string       `json:"added_tables,omitempty"`
	DroppedTables   []string       `json:"dropped_tables,omitempty"`
	AddedColumns    []ColumnChange `json:"added_columns,omitempty"`
	DroppedColumns  []ColumnChange `json:"dropped_columns,omitempty"`
	ModifiedColumns []ColumnChange `json:"modified_columns,omitempty"`
	AddedIndexes    []IndexChange  `json:"added_indexes,omitempty"`
	DroppedIndexes  []IndexChange  `json:"dropped_indexes,omitempty"`
}

type ColumnChange struct {
	Table   string `json:"table"`
	Column  string `json:"column"`
	OldType string `json:"old_type,omitempty"`
	NewType string `json:"new_type,omitempty"`
}

type IndexChange struct {
	Table string `json:"table"`
	Index string `json:"index"`
}

func (d *SchemaDiff) IsEmpty() bool {
	return len(d.AddedTables) == 0 && len(d.DroppedTables) == 0 &&
		len(d.AddedColumns) == 0 && len(d.DroppedColumns) == 0 &&
		len(d.ModifiedColumns) == 0 && len(d.AddedIndexes) == 0 &&
		len(d.DroppedIndexes) == 0
}

func (d *SchemaDiff) TotalChanges() int {
	return len(d.AddedTables) + len(d.DroppedTables) +
		len(d.AddedColumns) + len(d.DroppedColumns) +
		len(d.ModifiedColumns) + len(d.AddedIndexes) + len(d.DroppedIndexes)
}

type SchemaChangeInspector struct {
	store *store.Store
}

func NewSchemaChangeInspector(s *store.Store) *SchemaChangeInspector {
	return &SchemaChangeInspector{store: s}
}

func (sc *SchemaChangeInspector) Name() string { return "schema" }

func (sc *SchemaChangeInspector) Run(ctx context.Context, conn connector.Connector, cfg *config.Config) (*Result, error) {
	result := NewResult("schema")

	if !cfg.Checks.Schema.Enabled {
		return result.Finish(StatusSkipped, "schema change detection disabled"), nil
	}

	switch conn.Type() {
	case "mysql":
		return sc.runMySQL(ctx, conn, cfg, result)
	case "postgres":
		return sc.runPostgres(ctx, conn, cfg, result)
	case "redis":
		return result.Finish(StatusSkipped, "schema check not applicable to Redis"), nil
	default:
		return result.Finish(StatusSkipped, fmt.Sprintf("schema check not supported for %s", conn.Type())), nil
	}
}

func (sc *SchemaChangeInspector) runMySQL(ctx context.Context, conn connector.Connector, cfg *config.Config, result *Result) (*Result, error) {
	sqlConn, ok := conn.(connector.SQLConnector)
	if !ok {
		return result.Finish(StatusSkipped, "connector does not support SQL queries"), nil
	}

	var dbName string
	row := sqlConn.QueryRow(ctx, "SELECT DATABASE()")
	if err := row.Scan(&dbName); err != nil || dbName == "" {
		result.RiskScore = 20
		result.Details["error"] = "no database selected"
		return result.Finish(StatusWarning, "cannot determine current database for schema check"), nil
	}

	snapshot, err := sc.fetchMySQLSchema(ctx, sqlConn, dbName, cfg.Checks.Schema.ExcludeTables)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "access denied") ||
			strings.Contains(strings.ToLower(err.Error()), "permission") {
			result.RiskScore = 20
			result.Details["required_privilege"] = "SELECT on information_schema"
			return result.Finish(StatusWarning, "insufficient privilege for schema inspection"), nil
		}
		return nil, fmt.Errorf("failed to fetch MySQL schema: %w", err)
	}

	return sc.compareAndSave(ctx, conn, dbName, snapshot, result)
}

func (sc *SchemaChangeInspector) runPostgres(ctx context.Context, conn connector.Connector, cfg *config.Config, result *Result) (*Result, error) {
	sqlConn, ok := conn.(connector.SQLConnector)
	if !ok {
		return result.Finish(StatusSkipped, "connector does not support SQL queries"), nil
	}

	var dbName string
	row := sqlConn.QueryRow(ctx, "SELECT current_database()")
	if err := row.Scan(&dbName); err != nil {
		return nil, fmt.Errorf("failed to get current database: %w", err)
	}

	snapshot, err := sc.fetchPostgresSchema(ctx, sqlConn, cfg.Checks.Schema.ExcludeTables)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "permission") {
			result.RiskScore = 20
			result.Details["required_privilege"] = "SELECT on information_schema"
			return result.Finish(StatusWarning, "insufficient privilege for schema inspection"), nil
		}
		return nil, fmt.Errorf("failed to fetch PostgreSQL schema: %w", err)
	}

	return sc.compareAndSave(ctx, conn, dbName, snapshot, result)
}

func (sc *SchemaChangeInspector) fetchMySQLSchema(ctx context.Context, conn connector.SQLConnector, dbName string, excludeTables []string) (*SchemaSnapshot, error) {
	rows, err := conn.Query(ctx, `SELECT TABLE_NAME FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE' ORDER BY TABLE_NAME`, dbName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []TableSchema
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}
		if isExcluded(tableName, excludeTables) {
			continue
		}
		tables = append(tables, TableSchema{Name: tableName})
	}

	for i, table := range tables {
		cols, err := sc.fetchMySQLColumns(ctx, conn, dbName, table.Name)
		if err != nil {
			continue
		}
		tables[i].Columns = cols

		idxs, err := sc.fetchMySQLIndexes(ctx, conn, dbName, table.Name)
		if err != nil {
			continue
		}
		tables[i].Indexes = idxs
	}

	return &SchemaSnapshot{Tables: tables}, nil
}

func (sc *SchemaChangeInspector) fetchMySQLColumns(ctx context.Context, conn connector.SQLConnector, dbName, tableName string) ([]ColumnSchema, error) {
	rows, err := conn.Query(ctx, `SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COALESCE(COLUMN_DEFAULT, '')
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnSchema
	for rows.Next() {
		var c ColumnSchema
		var nullable string
		if err := rows.Scan(&c.Name, &c.Type, &nullable, &c.Default); err != nil {
			continue
		}
		c.Nullable = nullable == "YES"
		columns = append(columns, c)
	}
	return columns, nil
}

func (sc *SchemaChangeInspector) fetchMySQLIndexes(ctx context.Context, conn connector.SQLConnector, dbName, tableName string) ([]IndexSchema, error) {
	rows, err := conn.Query(ctx, `SELECT INDEX_NAME, COLUMN_NAME, NON_UNIQUE
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX`, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexMap := make(map[string]*IndexSchema)
	var indexOrder []string
	for rows.Next() {
		var indexName, colName string
		var nonUnique int
		if err := rows.Scan(&indexName, &colName, &nonUnique); err != nil {
			continue
		}
		if idx, ok := indexMap[indexName]; ok {
			idx.Columns = append(idx.Columns, colName)
		} else {
			indexMap[indexName] = &IndexSchema{
				Name:    indexName,
				Columns: []string{colName},
				Unique:  nonUnique == 0,
				Primary: indexName == "PRIMARY",
			}
			indexOrder = append(indexOrder, indexName)
		}
	}

	var indexes []IndexSchema
	for _, name := range indexOrder {
		indexes = append(indexes, *indexMap[name])
	}
	return indexes, nil
}

func (sc *SchemaChangeInspector) fetchPostgresSchema(ctx context.Context, conn connector.SQLConnector, excludeTables []string) (*SchemaSnapshot, error) {
	rows, err := conn.Query(ctx, `SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE' ORDER BY table_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []TableSchema
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}
		if isExcluded(tableName, excludeTables) {
			continue
		}
		tables = append(tables, TableSchema{Name: tableName})
	}

	for i, table := range tables {
		cols, err := sc.fetchPostgresColumns(ctx, conn, table.Name)
		if err != nil {
			continue
		}
		tables[i].Columns = cols

		idxs, err := sc.fetchPostgresIndexes(ctx, conn, table.Name)
		if err != nil {
			continue
		}
		tables[i].Indexes = idxs
	}

	return &SchemaSnapshot{Tables: tables}, nil
}

func (sc *SchemaChangeInspector) fetchPostgresColumns(ctx context.Context, conn connector.SQLConnector, tableName string) ([]ColumnSchema, error) {
	rows, err := conn.Query(ctx, `SELECT column_name, data_type, is_nullable, COALESCE(column_default, '')
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position`, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnSchema
	for rows.Next() {
		var c ColumnSchema
		var nullable string
		if err := rows.Scan(&c.Name, &c.Type, &nullable, &c.Default); err != nil {
			continue
		}
		c.Nullable = nullable == "YES"
		columns = append(columns, c)
	}
	return columns, nil
}

func (sc *SchemaChangeInspector) fetchPostgresIndexes(ctx context.Context, conn connector.SQLConnector, tableName string) ([]IndexSchema, error) {
	rows, err := conn.Query(ctx, `SELECT indexname, indexdef FROM pg_indexes
		WHERE schemaname = 'public' AND tablename = $1
		ORDER BY indexname`, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []IndexSchema
	for rows.Next() {
		var name, def string
		if err := rows.Scan(&name, &def); err != nil {
			continue
		}
		idx := IndexSchema{
			Name:    name,
			Unique:  strings.Contains(strings.ToUpper(def), "UNIQUE"),
			Primary: strings.HasSuffix(name, "_pkey"),
		}
		if colStart := strings.Index(def, "("); colStart > 0 {
			if colEnd := strings.LastIndex(def, ")"); colEnd > colStart {
				colStr := def[colStart+1 : colEnd]
				for _, col := range strings.Split(colStr, ",") {
					idx.Columns = append(idx.Columns, strings.TrimSpace(col))
				}
			}
		}
		indexes = append(indexes, idx)
	}
	return indexes, nil
}

func (sc *SchemaChangeInspector) compareAndSave(ctx context.Context, conn connector.Connector, dbName string, current *SchemaSnapshot, result *Result) (*Result, error) {
	previous, err := sc.store.GetLatestSchemaSnapshot(conn.Name(), dbName)
	if err != nil {
		result.Details["snapshot_error"] = err.Error()
	}

	snapshotJSON, err := json.Marshal(current)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema snapshot: %w", err)
	}

	if saveErr := sc.store.SaveSchemaSnapshot(&store.SchemaSnapshot{
		TargetName:   conn.Name(),
		TargetType:   conn.Type(),
		DatabaseName: dbName,
		SnapshotData: string(snapshotJSON),
		RunID:        "",
	}); saveErr != nil {
		result.Details["snapshot_save_error"] = saveErr.Error()
	}

	if previous == nil {
		result.Details["tables_count"] = len(current.Tables)
		return result.Finish(StatusSuccess, "initial schema snapshot captured"), nil
	}

	var oldSnapshot SchemaSnapshot
	if err := json.Unmarshal([]byte(previous.SnapshotData), &oldSnapshot); err != nil {
		result.Details["parse_error"] = err.Error()
		return result.Finish(StatusWarning, "failed to parse previous schema snapshot"), nil
	}

	diff := ComputeSchemaDiff(&oldSnapshot, current)
	result.Details["diff"] = diff
	result.Details["total_changes"] = diff.TotalChanges()

	if diff.IsEmpty() {
		return result.Finish(StatusSuccess, "no schema changes detected"), nil
	}

	result.RiskScore = computeSchemaRiskScore(diff)
	summary := formatSchemaDiffSummary(diff)
	if result.RiskScore >= 60 {
		return result.Finish(StatusError, summary), nil
	}
	if result.RiskScore >= 20 {
		return result.Finish(StatusWarning, summary), nil
	}
	return result.Finish(StatusSuccess, summary), nil
}

func ComputeSchemaDiff(old, new *SchemaSnapshot) *SchemaDiff {
	diff := &SchemaDiff{}

	oldTableMap := make(map[string]*TableSchema)
	for i := range old.Tables {
		oldTableMap[old.Tables[i].Name] = &old.Tables[i]
	}
	newTableMap := make(map[string]*TableSchema)
	for i := range new.Tables {
		newTableMap[new.Tables[i].Name] = &new.Tables[i]
	}

	for name := range newTableMap {
		if _, ok := oldTableMap[name]; !ok {
			diff.AddedTables = append(diff.AddedTables, name)
		}
	}
	for name := range oldTableMap {
		if _, ok := newTableMap[name]; !ok {
			diff.DroppedTables = append(diff.DroppedTables, name)
		}
	}

	for name, newTable := range newTableMap {
		oldTable, ok := oldTableMap[name]
		if !ok {
			continue
		}
		diffColumns(oldTable, newTable, diff)
		diffIndexes(oldTable, newTable, diff)
	}

	return diff
}

func diffColumns(old, new *TableSchema, diff *SchemaDiff) {
	oldCols := make(map[string]*ColumnSchema)
	for i := range old.Columns {
		oldCols[old.Columns[i].Name] = &old.Columns[i]
	}
	newCols := make(map[string]*ColumnSchema)
	for i := range new.Columns {
		newCols[new.Columns[i].Name] = &new.Columns[i]
	}

	for name, newCol := range newCols {
		oldCol, ok := oldCols[name]
		if !ok {
			diff.AddedColumns = append(diff.AddedColumns, ColumnChange{
				Table:   new.Name,
				Column:  name,
				NewType: newCol.Type,
			})
			continue
		}
		if oldCol.Type != newCol.Type {
			diff.ModifiedColumns = append(diff.ModifiedColumns, ColumnChange{
				Table:   new.Name,
				Column:  name,
				OldType: oldCol.Type,
				NewType: newCol.Type,
			})
		}
	}

	for name := range oldCols {
		if _, ok := newCols[name]; !ok {
			diff.DroppedColumns = append(diff.DroppedColumns, ColumnChange{
				Table:  old.Name,
				Column: name,
			})
		}
	}
}

func diffIndexes(old, new *TableSchema, diff *SchemaDiff) {
	oldIdxs := make(map[string]struct{})
	for _, idx := range old.Indexes {
		oldIdxs[idx.Name] = struct{}{}
	}
	newIdxs := make(map[string]struct{})
	for _, idx := range new.Indexes {
		newIdxs[idx.Name] = struct{}{}
	}

	for name := range newIdxs {
		if _, ok := oldIdxs[name]; !ok {
			diff.AddedIndexes = append(diff.AddedIndexes, IndexChange{Table: new.Name, Index: name})
		}
	}
	for name := range oldIdxs {
		if _, ok := newIdxs[name]; !ok {
			diff.DroppedIndexes = append(diff.DroppedIndexes, IndexChange{Table: old.Name, Index: name})
		}
	}
}

func computeSchemaRiskScore(diff *SchemaDiff) int {
	score := 0
	if len(diff.DroppedTables) > 0 {
		score = maxInt(score, 80)
	}
	if len(diff.DroppedColumns) > 0 {
		score = maxInt(score, 60)
	}
	if len(diff.ModifiedColumns) > 0 {
		score = maxInt(score, 40)
	}
	if len(diff.DroppedIndexes) > 0 {
		score = maxInt(score, 20)
	}
	if len(diff.AddedIndexes) > 0 {
		score = maxInt(score, 20)
	}
	if len(diff.AddedColumns) > 0 {
		score = maxInt(score, 10)
	}
	if len(diff.AddedTables) > 0 {
		score = maxInt(score, 10)
	}
	return score
}

func formatSchemaDiffSummary(diff *SchemaDiff) string {
	parts := []string{}
	if n := len(diff.AddedTables); n > 0 {
		parts = append(parts, fmt.Sprintf("%d table(s) added", n))
	}
	if n := len(diff.DroppedTables); n > 0 {
		parts = append(parts, fmt.Sprintf("%d table(s) dropped", n))
	}
	if n := len(diff.AddedColumns); n > 0 {
		parts = append(parts, fmt.Sprintf("%d column(s) added", n))
	}
	if n := len(diff.DroppedColumns); n > 0 {
		parts = append(parts, fmt.Sprintf("%d column(s) dropped", n))
	}
	if n := len(diff.ModifiedColumns); n > 0 {
		parts = append(parts, fmt.Sprintf("%d column(s) modified", n))
	}
	if n := len(diff.AddedIndexes); n > 0 {
		parts = append(parts, fmt.Sprintf("%d index(es) added", n))
	}
	if n := len(diff.DroppedIndexes); n > 0 {
		parts = append(parts, fmt.Sprintf("%d index(es) dropped", n))
	}
	return fmt.Sprintf("schema changes detected: %s", strings.Join(parts, ", "))
}

func isExcluded(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(name, prefix) {
				return true
			}
		} else if name == pattern {
			return true
		}
	}
	return false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
