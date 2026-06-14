package config

import "time"

type Config struct {
	Version       string             `yaml:"version"`
	Global        GlobalConfig       `yaml:"global"`
	Targets       []Target           `yaml:"targets"`
	Checks        ChecksConfig       `yaml:"checks"`
	Risk          RiskConfig         `yaml:"risk"`
	Plugins       PluginsConfig      `yaml:"plugins"`
	Report        ReportConfig       `yaml:"report"`
	Notifications NotificationConfig `yaml:"notifications"`
}

type NotificationConfig struct {
	Enabled    bool                   `yaml:"enabled"`
	Thresholds NotificationThresholds `yaml:"thresholds"`
	Channels   []ChannelConfig        `yaml:"channels"`
}

type NotificationThresholds struct {
	GlobalRiskScore int            `yaml:"global_risk_score"`
	Category        map[string]int `yaml:"category"`
}

type ChannelConfig struct {
	Type         string            `yaml:"type"`
	URL          string            `yaml:"url"`
	Timeout      time.Duration     `yaml:"timeout"`
	Headers      map[string]string `yaml:"headers"`
	SlackChannel string            `yaml:"slack_channel"`
	SMTPHost     string            `yaml:"smtp_host"`
	SMTPPort     int               `yaml:"smtp_port"`
	From         string            `yaml:"from"`
	To           []string          `yaml:"to"`
	Username     string            `yaml:"username"`
	Password     string            `yaml:"password"`
	Path         string            `yaml:"path"`
}

type GlobalConfig struct {
	Timeout       time.Duration `yaml:"timeout"`
	MaxRetries    int           `yaml:"max_retries"`
	RetryInterval time.Duration `yaml:"retry_interval"`
	LogLevel      string        `yaml:"log_level"`
	DataDir       string        `yaml:"data_dir"`
	ReportDir     string        `yaml:"report_dir"`
}

type Target struct {
	Name     string            `yaml:"name"`
	Type     string            `yaml:"type"`
	Host     string            `yaml:"host"`
	Port     int               `yaml:"port"`
	User     string            `yaml:"user"`
	Password string            `yaml:"password"`
	Database string            `yaml:"database"`
	SSLMode  string            `yaml:"ssl_mode"`
	DB       int               `yaml:"db"`
	Params   map[string]string `yaml:"params"`
}

type ChecksConfig struct {
	SlowQuery   SlowQueryConfig   `yaml:"slowquery"`
	Capacity    CapacityConfig    `yaml:"capacity"`
	Index       IndexConfig       `yaml:"index"`
	Backup      BackupConfig      `yaml:"backup"`
	Permission  PermissionConfig  `yaml:"permission"`
	Replication ReplicationConfig `yaml:"replication"`
	Schema      SchemaConfig      `yaml:"schema"`
}

type ReplicationConfig struct {
	Enabled            bool `yaml:"enabled"`
	MaxLagSeconds      int  `yaml:"max_lag_seconds"`
	CriticalLagSeconds int  `yaml:"critical_lag_seconds"`
}

type SchemaConfig struct {
	Enabled       bool     `yaml:"enabled"`
	ExcludeTables []string `yaml:"exclude_tables"`
}

type SlowQueryConfig struct {
	Enabled   bool          `yaml:"enabled"`
	Threshold time.Duration `yaml:"threshold"`
	TopN      int           `yaml:"top_n"`
}

type CapacityConfig struct {
	Enabled             bool          `yaml:"enabled"`
	WarnThresholdGB     float64       `yaml:"warn_threshold_gb"`
	CriticalThresholdGB float64       `yaml:"critical_threshold_gb"`
	ScanTimeout         time.Duration `yaml:"scan_timeout"`
}

type IndexConfig struct {
	Enabled      bool `yaml:"enabled"`
	MinTableRows int  `yaml:"min_table_rows"`
}

type BackupConfig struct {
	Enabled bool          `yaml:"enabled"`
	Paths   []string      `yaml:"paths"`
	MaxAge  time.Duration `yaml:"max_age"`
}

type PermissionConfig struct {
	Enabled      bool     `yaml:"enabled"`
	DenyPatterns []string `yaml:"deny_patterns"`
}

type RiskConfig struct {
	Weights map[string]int `yaml:"weights"`
}

type PluginsConfig struct {
	Dir     string   `yaml:"dir"`
	Enabled []string `yaml:"enabled"`
}

type ReportConfig struct {
	Format         string `yaml:"format"`
	IncludeDetails bool   `yaml:"include_details"`
}
