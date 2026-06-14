package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Liyonglin20030201/task061402/internal/connector"
	"github.com/Liyonglin20030201/task061402/internal/inspector"
	"github.com/Liyonglin20030201/task061402/internal/plugin"
	"github.com/Liyonglin20030201/task061402/internal/store"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage and run plugins",
	Long:  "List available plugins or run a specific plugin against target databases.",
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available plugins",
	RunE: func(cmd *cobra.Command, args []string) error {
		pluginDir := cfg.Plugins.Dir
		plugins, err := plugin.Discover(pluginDir)
		if err != nil {
			return fmt.Errorf("failed to discover plugins: %w", err)
		}

		if len(plugins) == 0 {
			fmt.Printf("No plugins found in %s\n", pluginDir)
			return nil
		}

		fmt.Printf("Available plugins (%s):\n", pluginDir)
		for _, p := range plugins {
			fmt.Printf("  %-20s %s (%s)\n", p.Name, p.Description, p.Type)
		}

		return nil
	},
}

var pluginRunName string

var pluginRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a specific plugin",
	RunE: func(cmd *cobra.Command, args []string) error {
		if pluginRunName == "" && len(args) > 0 {
			pluginRunName = args[0]
		}
		if pluginRunName == "" {
			return fmt.Errorf("plugin name required: use --name or provide as argument")
		}

		plugins, err := plugin.Discover(cfg.Plugins.Dir)
		if err != nil {
			return fmt.Errorf("failed to discover plugins: %w", err)
		}

		var targetPlugin *plugin.PluginInfo
		for _, p := range plugins {
			if p.Name == pluginRunName {
				targetPlugin = &p
				break
			}
		}

		if targetPlugin == nil {
			return fmt.Errorf("plugin %q not found in %s", pluginRunName, cfg.Plugins.Dir)
		}

		targets := resolveTargets()
		executor := plugin.NewExecutor(cfg.Plugins.Dir)

		for _, target := range targets {
			ctx, cancel := context.WithTimeout(context.Background(), cfg.Global.Timeout)

			conn, err := connector.NewFromTarget(target)
			if err != nil {
				cancel()
				continue
			}

			if err := conn.Connect(ctx); err != nil {
				fmt.Printf("  ✗ %s: connection failed - %v\n", target.Name, err)
				cancel()
				continue
			}

			result, err := executor.Run(ctx, *targetPlugin, conn, cfg)
			conn.Close()
			cancel()

			if err != nil {
				log.Error(fmt.Sprintf("[%s] plugin %s error: %v", target.Name, pluginRunName, err))
				continue
			}

			db.SaveInspection(&store.Inspection{
				RunID:      runID,
				TargetName: target.Name,
				TargetType: target.Type,
				CheckType:  result.CheckType,
				Status:     string(result.Status),
				RiskScore:  result.RiskScore,
				Summary:    result.Summary,
				Details:    result.Details,
				StartedAt:  result.StartedAt,
				FinishedAt: result.FinishedAt,
			})

			printResult(target.Name, result)
		}

		return nil
	},
}

func init() {
	pluginRunCmd.Flags().StringVar(&pluginRunName, "name", "", "plugin name to run")
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginRunCmd)
	rootCmd.AddCommand(pluginCmd)
}

var _ inspector.Inspector = (*inspector.PingInspector)(nil)
