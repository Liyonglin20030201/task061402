package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	resolved := resolveEnvVars(string(data))

	cfg := &Config{}
	if err := yaml.Unmarshal([]byte(resolved), cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	applyDefaults(cfg)

	if errs := Validate(cfg); len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		return nil, fmt.Errorf("config validation failed:\n  %s", strings.Join(msgs, "\n  "))
	}

	return cfg, nil
}

func resolveEnvVars(content string) string {
	return envVarPattern.ReplaceAllStringFunc(content, func(match string) string {
		varName := envVarPattern.FindStringSubmatch(match)[1]
		if val, ok := os.LookupEnv(varName); ok {
			return val
		}
		return match
	})
}

func applyDefaults(cfg *Config) {
	if cfg.Global.Timeout == 0 {
		cfg.Global.Timeout = 30 * time.Second
	}
	if cfg.Global.MaxRetries == 0 {
		cfg.Global.MaxRetries = 3
	}
	if cfg.Global.RetryInterval == 0 {
		cfg.Global.RetryInterval = 5 * time.Second
	}
	if cfg.Global.LogLevel == "" {
		cfg.Global.LogLevel = "info"
	}
	if cfg.Global.DataDir == "" {
		cfg.Global.DataDir = "./data"
	}
	if cfg.Global.ReportDir == "" {
		cfg.Global.ReportDir = "./reports"
	}
	if cfg.Checks.SlowQuery.TopN == 0 {
		cfg.Checks.SlowQuery.TopN = 20
	}
	if cfg.Checks.SlowQuery.Threshold == 0 {
		cfg.Checks.SlowQuery.Threshold = time.Second
	}
	if cfg.Checks.Index.MinTableRows == 0 {
		cfg.Checks.Index.MinTableRows = 1000
	}
	if cfg.Checks.Capacity.ScanTimeout == 0 {
		cfg.Checks.Capacity.ScanTimeout = 2 * time.Minute
	}
	if cfg.Checks.Backup.MaxAge == 0 {
		cfg.Checks.Backup.MaxAge = 24 * time.Hour
	}
	if cfg.Report.Format == "" {
		cfg.Report.Format = "html"
	}
	if cfg.Checks.Replication.MaxLagSeconds == 0 {
		cfg.Checks.Replication.MaxLagSeconds = 30
	}
	if cfg.Checks.Replication.CriticalLagSeconds == 0 {
		cfg.Checks.Replication.CriticalLagSeconds = 120
	}
	if cfg.Notifications.Thresholds.GlobalRiskScore == 0 {
		cfg.Notifications.Thresholds.GlobalRiskScore = 60
	}
}
