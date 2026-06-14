package cmd

import (
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
			if globalCtx.Err() != nil {
				fmt.Printf("  ⚠ interrupted, stopping further checks\n")
				break
			}

			conn, err := connector.NewFromTarget(target)
			if err != nil {
				log.Error(fmt.Sprintf("[%s] connector error: %v", target.Name, err))
				continue
			}

			// backup 检查不强制要求连接成功
			if err := connectWithRetry(globalCtx, conn, target); err != nil {
				fmt.Printf("  ⚠ %s: running backup check without connection\n", target.Name)
			}

			checkCtx, checkCancel := createCheckContext(globalCtx, cfg.Global.Timeout)
			result, err := insp.Run(checkCtx, conn, cfg)
			checkCancel()
			conn.Close()

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
