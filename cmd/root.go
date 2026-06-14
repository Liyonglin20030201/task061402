package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	cfg       *config.Config
	db        *store.Store
	log       *logger.Logger
	runID     string
	globalCtx context.Context
	globalCancel context.CancelFunc
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

		// 捕获 OS 信号，确保 Ctrl+C 能中断卡死的连接
		globalCtx, globalCancel = context.WithCancel(context.Background())
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			sig := <-sigCh
			fmt.Fprintf(os.Stderr, "\nReceived signal %v, shutting down gracefully...\n", sig)
			globalCancel()
			// 二次信号强制退出
			<-sigCh
			os.Exit(130)
		}()

		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}

		// 应用 --timeout 覆盖
		if timeout != "" {
			d, err := time.ParseDuration(timeout)
			if err != nil {
				return fmt.Errorf("invalid --timeout value %q: %w", timeout, err)
			}
			cfg.Global.Timeout = d
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
		if globalCancel != nil {
			globalCancel()
		}
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

// connectWithRetry 连接数据库，带超时和重试机制。超时或信号中断时立即返回错误而非卡死。
func connectWithRetry(ctx context.Context, conn interface{ Connect(context.Context) error; Name() string; Close() error }, target config.Target) error {
	var lastErr error
	for attempt := 1; attempt <= cfg.Global.MaxRetries; attempt++ {
		connCtx, connCancel := context.WithTimeout(ctx, cfg.Global.Timeout)

		err := conn.Connect(connCtx)
		connCancel()

		if err == nil {
			return nil
		}
		lastErr = err

		if ctx.Err() != nil {
			return fmt.Errorf("interrupted while connecting to %s: %w", target.Name, ctx.Err())
		}

		if attempt < cfg.Global.MaxRetries {
			log.Warn(fmt.Sprintf("[%s] connection attempt %d/%d failed: %v, retrying in %s...",
				target.Name, attempt, cfg.Global.MaxRetries, err, cfg.Global.RetryInterval))
			fmt.Fprintf(os.Stderr, "  ⚠ %s: attempt %d/%d failed, retrying...\n",
				target.Name, attempt, cfg.Global.MaxRetries)

			select {
			case <-time.After(cfg.Global.RetryInterval):
			case <-ctx.Done():
				return fmt.Errorf("interrupted while waiting to retry %s: %w", target.Name, ctx.Err())
			}
		}
	}
	return fmt.Errorf("connection to %s failed after %d attempts: %w", target.Name, cfg.Global.MaxRetries, lastErr)
}

// createCheckContext 为单个检查创建带超时的 context，同时继承全局取消信号。
func createCheckContext(parent context.Context, checkTimeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, checkTimeout)
}
