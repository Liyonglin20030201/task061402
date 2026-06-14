package config

import (
	"fmt"
	"strings"
)

func Validate(cfg *Config) []error {
	var errs []error

	if len(cfg.Targets) == 0 {
		errs = append(errs, fmt.Errorf("at least one target must be defined"))
	}

	validTypes := map[string]bool{"mysql": true, "postgres": true, "redis": true}
	targetNames := make(map[string]bool)

	for i, t := range cfg.Targets {
		prefix := fmt.Sprintf("targets[%d](%s)", i, t.Name)

		if t.Name == "" {
			prefix = fmt.Sprintf("targets[%d]", i)
			errs = append(errs, fmt.Errorf("%s.name: must not be empty", prefix))
		} else if targetNames[t.Name] {
			errs = append(errs, fmt.Errorf("%s.name: duplicate target name %q", prefix, t.Name))
		} else {
			targetNames[t.Name] = true
		}

		if t.Type == "" {
			errs = append(errs, fmt.Errorf("%s.type: must not be empty, valid options are: mysql, postgres, redis", prefix))
		} else if !validTypes[t.Type] {
			errs = append(errs, fmt.Errorf("%s.type: invalid value %q, must be one of: mysql, postgres, redis", prefix, t.Type))
		}

		if t.Host == "" {
			errs = append(errs, fmt.Errorf("%s.host: must not be empty", prefix))
		}

		if t.Port < 1 || t.Port > 65535 {
			errs = append(errs, fmt.Errorf("%s.port: must be between 1 and 65535; got %d", prefix, t.Port))
		}
	}

	if cfg.Global.Timeout < 0 {
		errs = append(errs, fmt.Errorf("global.timeout: must be positive"))
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[strings.ToLower(cfg.Global.LogLevel)] {
		errs = append(errs, fmt.Errorf("global.log_level: must be one of debug, info, warn, error; got %q", cfg.Global.LogLevel))
	}

	if cfg.Checks.Capacity.Enabled {
		if cfg.Checks.Capacity.WarnThresholdGB >= cfg.Checks.Capacity.CriticalThresholdGB && cfg.Checks.Capacity.CriticalThresholdGB > 0 {
			errs = append(errs, fmt.Errorf("checks.capacity: warn_threshold_gb must be less than critical_threshold_gb"))
		}
	}

	if len(cfg.Risk.Weights) > 0 {
		total := 0
		for _, w := range cfg.Risk.Weights {
			if w < 0 {
				errs = append(errs, fmt.Errorf("risk.weights: values must be non-negative"))
				break
			}
			total += w
		}
		if total != 100 {
			errs = append(errs, fmt.Errorf("risk.weights: values must sum to 100; got %d", total))
		}
	}

	validFormats := map[string]bool{"html": true, "json": true, "csv": true}
	if !validFormats[cfg.Report.Format] {
		errs = append(errs, fmt.Errorf("report.format: must be one of html, json, csv; got %q", cfg.Report.Format))
	}

	return errs
}
