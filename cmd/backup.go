package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Liyonglin20030201/task061402/internal/connector"
	"github.com/Liyonglin20030201/task061402/internal/inspector"
	"github.com/Liyonglin20030201/task061402/internal/store"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Verify database backups",
	Long:  "Check backup file existence, freshness, and integrity based on configured paths.",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets := resolveTargets()
		insp := inspector.NewBackupInspector()

		for _, target := range targets {
			ctx, cancel := context.WithTimeout(context.Background(), cfg.Global.Timeout)

			conn, err := connector.NewFromTarget(target)
			if err != nil {
				cancel()
				continue
			}

			if err := conn.Connect(ctx); err != nil {
				fmt.Printf("  ⚠ %s: running backup check without connection\n", target.Name)
			}

			result, err := insp.Run(ctx, conn, cfg)
			if conn != nil {
				conn.Close()
			}
			cancel()

			if err != nil {
				log.Error(fmt.Sprintf("[%s] backup check error: %v", target.Name, err))
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

		fmt.Printf("\nRun ID: %s\n", runID)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(backupCmd)
}
