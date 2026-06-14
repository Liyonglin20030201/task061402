package cmd

import (
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/Liyonglin20030201/task061402/internal/config"
	"github.com/Liyonglin20030201/task061402/internal/logger"
	"github.com/Liyonglin20030201/task061402/internal/store"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

var (
	cfgFile    string
	targetName string
	verbose    bool
	logLevel   string
	timeout    string

	cfg    *config.Config
	db     *store.Store
	log    *logger.Logger
	runID  string
)

var rootCmd = &cobra.Command{
	Use:   "dbinspect",
	Short: "Database inspection tool for MySQL, PostgreSQL, and Redis",
	Long: `dbinspect is a comprehensive database inspection CLI tool that supports
connection testing, slow query analysis, capacity statistics, index suggestions,
backup verification, permission scanning, risk scoring, and report generation.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "version" || cmd.Name() == "help" {
			return nil
		}

		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}

		if logLevel != "" {
			cfg.Global.LogLevel = logLevel
		}

		db, err = store.New(cfg.Global.DataDir)
		if err != nil {
			return fmt.Errorf("failed to initialize store: %w", err)
		}

		runID = uuid.New().String()
		log = logger.New(logger.ParseLevel(cfg.Global.LogLevel), runID, db)
		log.Info(fmt.Sprintf("starting inspection run %s", runID))

		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if db != nil {
			db.Close()
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "dbinspect.yaml", "config file path")
	rootCmd.PersistentFlags().StringVarP(&targetName, "target", "t", "", "target database name from config")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "log level: debug|info|warn|error")
	rootCmd.PersistentFlags().StringVar(&timeout, "timeout", "", "global timeout override (e.g., 60s)")
}

func resolveTargets() []config.Target {
	if targetName != "" {
		for _, t := range cfg.Targets {
			if t.Name == targetName {
				return []config.Target{t}
			}
		}
		fmt.Fprintf(os.Stderr, "Error: target %q not found in config\n", targetName)
		fmt.Fprintf(os.Stderr, "Available targets: ")
		for i, t := range cfg.Targets {
			if i > 0 {
				fmt.Fprintf(os.Stderr, ", ")
			}
			fmt.Fprintf(os.Stderr, "%s (%s)", t.Name, t.Type)
		}
		fmt.Fprintln(os.Stderr)
		os.Exit(1)
	}
	return cfg.Targets
}
