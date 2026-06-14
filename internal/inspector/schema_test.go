package inspector

import (
	"testing"
)

func TestSchemaChangeInspector_Name(t *testing.T) {
	insp := NewSchemaChangeInspector(nil)
	if insp.Name() != "schema" {
		t.Errorf("expected name 'schema', got %q", insp.Name())
	}
}

func TestComputeSchemaDiff_NoChanges(t *testing.T) {
	old := &SchemaSnapshot{
		Tables: []TableSchema{
			{Name: "users", Columns: []ColumnSchema{{Name: "id", Type: "int"}}, Indexes: []IndexSchema{{Name: "PRIMARY", Columns: []string{"id"}}}},
		},
	}
	diff := ComputeSchemaDiff(old, old)
	if !diff.IsEmpty() {
		t.Error("expected empty diff for identical schemas")
	}
}

func TestComputeSchemaDiff_AddedTable(t *testing.T) {
	old := &SchemaSnapshot{Tables: []TableSchema{{Name: "users"}}}
	new := &SchemaSnapshot{Tables: []TableSchema{{Name: "users"}, {Name: "orders"}}}
	diff := ComputeSchemaDiff(old, new)
	if len(diff.AddedTables) != 1 || diff.AddedTables[0] != "orders" {
		t.Errorf("expected 1 added table 'orders', got %v", diff.AddedTables)
	}
}

func TestComputeSchemaDiff_DroppedTable(t *testing.T) {
	old := &SchemaSnapshot{Tables: []TableSchema{{Name: "users"}, {Name: "orders"}}}
	new := &SchemaSnapshot{Tables: []TableSchema{{Name: "users"}}}
	diff := ComputeSchemaDiff(old, new)
	if len(diff.DroppedTables) != 1 || diff.DroppedTables[0] != "orders" {
		t.Errorf("expected 1 dropped table 'orders', got %v", diff.DroppedTables)
	}
}

func TestComputeSchemaDiff_AddedColumn(t *testing.T) {
	old := &SchemaSnapshot{Tables: []TableSchema{
		{Name: "users", Columns: []ColumnSchema{{Name: "id", Type: "int"}}},
	}}
	new := &SchemaSnapshot{Tables: []TableSchema{
		{Name: "users", Columns: []ColumnSchema{{Name: "id", Type: "int"}, {Name: "email", Type: "varchar(255)"}}},
	}}
	diff := ComputeSchemaDiff(old, new)
	if len(diff.AddedColumns) != 1 {
		t.Fatalf("expected 1 added column, got %d", len(diff.AddedColumns))
	}
	if diff.AddedColumns[0].Column != "email" {
		t.Errorf("expected added column 'email', got %q", diff.AddedColumns[0].Column)
	}
}

func TestComputeSchemaDiff_DroppedColumn(t *testing.T) {
	old := &SchemaSnapshot{Tables: []TableSchema{
		{Name: "users", Columns: []ColumnSchema{{Name: "id", Type: "int"}, {Name: "tmp", Type: "text"}}},
	}}
	new := &SchemaSnapshot{Tables: []TableSchema{
		{Name: "users", Columns: []ColumnSchema{{Name: "id", Type: "int"}}},
	}}
	diff := ComputeSchemaDiff(old, new)
	if len(diff.DroppedColumns) != 1 || diff.DroppedColumns[0].Column != "tmp" {
		t.Errorf("expected 1 dropped column 'tmp', got %v", diff.DroppedColumns)
	}
}

func TestComputeSchemaDiff_ModifiedColumn(t *testing.T) {
	old := &SchemaSnapshot{Tables: []TableSchema{
		{Name: "users", Columns: []ColumnSchema{{Name: "name", Type: "varchar(50)"}}},
	}}
	new := &SchemaSnapshot{Tables: []TableSchema{
		{Name: "users", Columns: []ColumnSchema{{Name: "name", Type: "varchar(255)"}}},
	}}
	diff := ComputeSchemaDiff(old, new)
	if len(diff.ModifiedColumns) != 1 {
		t.Fatalf("expected 1 modified column, got %d", len(diff.ModifiedColumns))
	}
	if diff.ModifiedColumns[0].OldType != "varchar(50)" || diff.ModifiedColumns[0].NewType != "varchar(255)" {
		t.Errorf("unexpected modification: %+v", diff.ModifiedColumns[0])
	}
}

func TestComputeSchemaDiff_IndexChanges(t *testing.T) {
	old := &SchemaSnapshot{Tables: []TableSchema{
		{Name: "users", Indexes: []IndexSchema{{Name: "idx_name"}, {Name: "idx_old"}}},
	}}
	new := &SchemaSnapshot{Tables: []TableSchema{
		{Name: "users", Indexes: []IndexSchema{{Name: "idx_name"}, {Name: "idx_new"}}},
	}}
	diff := ComputeSchemaDiff(old, new)
	if len(diff.AddedIndexes) != 1 || diff.AddedIndexes[0].Index != "idx_new" {
		t.Errorf("expected 1 added index 'idx_new', got %v", diff.AddedIndexes)
	}
	if len(diff.DroppedIndexes) != 1 || diff.DroppedIndexes[0].Index != "idx_old" {
		t.Errorf("expected 1 dropped index 'idx_old', got %v", diff.DroppedIndexes)
	}
}

func TestComputeSchemaRiskScore(t *testing.T) {
	tests := []struct {
		name     string
		diff     *SchemaDiff
		expected int
	}{
		{"no changes", &SchemaDiff{}, 0},
		{"added table", &SchemaDiff{AddedTables: []string{"t"}}, 10},
		{"added column", &SchemaDiff{AddedColumns: []ColumnChange{{Table: "t", Column: "c"}}}, 10},
		{"dropped index", &SchemaDiff{DroppedIndexes: []IndexChange{{Table: "t", Index: "i"}}}, 20},
		{"modified column", &SchemaDiff{ModifiedColumns: []ColumnChange{{Table: "t", Column: "c"}}}, 40},
		{"dropped column", &SchemaDiff{DroppedColumns: []ColumnChange{{Table: "t", Column: "c"}}}, 60},
		{"dropped table", &SchemaDiff{DroppedTables: []string{"t"}}, 80},
		{"mixed high", &SchemaDiff{DroppedTables: []string{"t"}, AddedColumns: []ColumnChange{{Table: "t", Column: "c"}}}, 80},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeSchemaRiskScore(tt.diff)
			if got != tt.expected {
				t.Errorf("computeSchemaRiskScore() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestIsExcluded(t *testing.T) {
	tests := []struct {
		name     string
		table    string
		patterns []string
		expected bool
	}{
		{"exact match", "tmp_data", []string{"tmp_data"}, true},
		{"prefix wildcard", "tmp_data", []string{"tmp_*"}, true},
		{"no match", "users", []string{"tmp_*"}, false},
		{"empty patterns", "users", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isExcluded(tt.table, tt.patterns)
			if got != tt.expected {
				t.Errorf("isExcluded(%q, %v) = %v, want %v", tt.table, tt.patterns, got, tt.expected)
			}
		})
	}
}

func TestSchemaDiff_TotalChanges(t *testing.T) {
	diff := &SchemaDiff{
		AddedTables:     []string{"a", "b"},
		DroppedColumns:  []ColumnChange{{Table: "t", Column: "c"}},
		ModifiedColumns: []ColumnChange{{Table: "t", Column: "d"}},
	}
	if diff.TotalChanges() != 4 {
		t.Errorf("expected 4 total changes, got %d", diff.TotalChanges())
	}
}
