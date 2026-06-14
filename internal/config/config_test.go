package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadValidConfig(t *testing.T) {
	content := `
version: "1"
global:
  timeout: 10s
  log_level: info
  data_dir: ./data
  report_dir: ./reports
targets:
  - name: test-db
    type: mysql
    host: localhost
    port: 3306
    user: root
    password: pass
    database: test
checks:
  slowquery:
    enabled: true
    threshold: 1s
    top_n: 10
  capacity:
    enabled: true
    warn_threshold_gb: 10
    critical_threshold_gb: 50
  index:
    enabled: true
    min_table_rows: 100
  backup:
    enabled: false
    paths: []
    max_age: 24h
  permission:
    enabled: true
    deny_patterns:
      - "SUPER"
risk:
  weights:
    connection: 20
    slowquery: 20
    capacity: 15
    index: 15
    backup: 15
    permission: 15
plugins:
  dir: ./plugins
  enabled: []
report:
  format: html
  include_details: true
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(cfgPath, []byte(content), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Global.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", cfg.Global.Timeout)
	}

	if len(cfg.Targets) != 1 {
		t.Errorf("expected 1 target, got %d", len(cfg.Targets))
	}

	if cfg.Targets[0].Name != "test-db" {
		t.Errorf("expected target name 'test-db', got %q", cfg.Targets[0].Name)
	}
}

func TestLoadInvalidConfig(t *testing.T) {
	content := `
version: "1"
global:
  timeout: 10s
  log_level: invalid
targets: []
risk:
  weights:
    connection: 50
report:
  format: pdf
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(cfgPath, []byte(content), 0644)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestEnvVarSubstitution(t *testing.T) {
	os.Setenv("TEST_DB_PASSWORD", "secret123")
	defer os.Unsetenv("TEST_DB_PASSWORD")

	content := `
version: "1"
global:
  timeout: 5s
  log_level: info
  data_dir: ./data
  report_dir: ./reports
targets:
  - name: test-db
    type: mysql
    host: localhost
    port: 3306
    user: root
    password: "${TEST_DB_PASSWORD}"
    database: test
checks:
  slowquery:
    enabled: false
  capacity:
    enabled: false
  index:
    enabled: false
  backup:
    enabled: false
    paths: []
    max_age: 24h
  permission:
    enabled: false
risk:
  weights:
    connection: 20
    slowquery: 20
    capacity: 15
    index: 15
    backup: 15
    permission: 15
plugins:
  dir: ./plugins
  enabled: []
report:
  format: json
  include_details: true
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(cfgPath, []byte(content), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Targets[0].Password != "secret123" {
		t.Errorf("expected password 'secret123', got %q", cfg.Targets[0].Password)
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &Config{
				Global: GlobalConfig{Timeout: 10 * time.Second, LogLevel: "info"},
				Targets: []Target{
					{Name: "db1", Type: "mysql", Host: "localhost", Port: 3306},
				},
				Risk:   RiskConfig{Weights: map[string]int{"connection": 20, "slowquery": 20, "capacity": 15, "index": 15, "backup": 15, "permission": 15}},
				Report: ReportConfig{Format: "html"},
			},
			wantErr: false,
		},
		{
			name: "invalid target type",
			cfg: &Config{
				Global: GlobalConfig{Timeout: 10 * time.Second, LogLevel: "info"},
				Targets: []Target{
					{Name: "db1", Type: "oracle", Host: "localhost", Port: 1521},
				},
				Risk:   RiskConfig{Weights: map[string]int{"connection": 100}},
				Report: ReportConfig{Format: "html"},
			},
			wantErr: true,
		},
		{
			name: "empty targets",
			cfg: &Config{
				Global:  GlobalConfig{Timeout: 10 * time.Second, LogLevel: "info"},
				Targets: []Target{},
				Report:  ReportConfig{Format: "html"},
			},
			wantErr: true,
		},
		{
			name: "duplicate target names",
			cfg: &Config{
				Global: GlobalConfig{Timeout: 10 * time.Second, LogLevel: "info"},
				Targets: []Target{
					{Name: "db1", Type: "mysql", Host: "localhost", Port: 3306},
					{Name: "db1", Type: "postgres", Host: "localhost", Port: 5432},
				},
				Risk:   RiskConfig{Weights: map[string]int{"connection": 100}},
				Report: ReportConfig{Format: "html"},
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			cfg: &Config{
				Global: GlobalConfig{Timeout: 10 * time.Second, LogLevel: "info"},
				Targets: []Target{
					{Name: "db1", Type: "mysql", Host: "localhost", Port: 99999},
				},
				Risk:   RiskConfig{Weights: map[string]int{"connection": 100}},
				Report: ReportConfig{Format: "html"},
			},
			wantErr: true,
		},
		{
			name: "empty target type",
			cfg: &Config{
				Global: GlobalConfig{Timeout: 10 * time.Second, LogLevel: "info"},
				Targets: []Target{
					{Name: "db1", Type: "", Host: "localhost", Port: 3306},
				},
				Risk:   RiskConfig{Weights: map[string]int{"connection": 100}},
				Report: ReportConfig{Format: "html"},
			},
			wantErr: true,
		},
		{
			name: "typo in target type",
			cfg: &Config{
				Global: GlobalConfig{Timeout: 10 * time.Second, LogLevel: "info"},
				Targets: []Target{
					{Name: "db1", Type: "mysq", Host: "localhost", Port: 3306},
				},
				Risk:   RiskConfig{Weights: map[string]int{"connection": 100}},
				Report: ReportConfig{Format: "html"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Validate(tt.cfg)
			if tt.wantErr && len(errs) == 0 {
				t.Error("expected validation errors, got none")
			}
			if !tt.wantErr && len(errs) > 0 {
				t.Errorf("expected no errors, got: %v", errs)
			}
		})
	}
}
