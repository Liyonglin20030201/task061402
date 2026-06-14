package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Liyonglin20030201/task061402/internal/connector"
	"github.com/Liyonglin20030201/task061402/internal/inspector"
	"github.com/Liyonglin20030201/task061402/internal/store"
)

var capacityCmd = &cobra.Command{
	Use:   "capacity",
	Short: "Show database capacity statistics",
	Long:  "Analyze database and table sizes, memory usage, and storage capacity.",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets := resolveTargets()
		insp := inspector.NewCapacityInspector()

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

			if err := connectWithRetry(globalCtx, conn, target); err != nil {
				log.Error(fmt.Sprintf("[%s] %v", target.Name, err))
				fmt.Printf("  ✗ %s: %v\n", target.Name, err)
				continue
			}

			// capacity 使用自己的 scan_timeout（在 inspector 内部管理），
			// 但外层仍传入 globalCtx 以响应信号中断
			result, err := insp.Run(globalCtx, conn, cfg)
			conn.Close()

			if err != nil {
				log.Error(fmt.Sprintf("[%s] capacity check error: %v", target.Name, err))
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

			if verbose {
				if totalGB, ok := result.Details["total_size_gb"]; ok {
					fmt.Printf("    Total size: %v GB\n", totalGB)
				}
				if _, ok := result.Details["timeout"]; ok {
					fmt.Printf("    ⚠ Scan was terminated due to timeout (scan_timeout: %s)\n",
						cfg.Checks.Capacity.ScanTimeout)
				}
			}
		}

		fmt.Printf("\nRun ID: %s\n", runID)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(capacityCmd)
}
