package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Liyonglin20030201/task061402/internal/connector"
	"github.com/Liyonglin20030201/task061402/internal/inspector"
	"github.com/Liyonglin20030201/task061402/internal/store"
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Analyze database indexes",
	Long:  "Check for duplicate, unused, or missing indexes and provide recommendations.",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets := resolveTargets()
		insp := inspector.NewIndexInspector()

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

			checkCtx, checkCancel := createCheckContext(globalCtx, cfg.Global.Timeout)
			result, err := insp.Run(checkCtx, conn, cfg)
			checkCancel()
			conn.Close()

			if err != nil {
				log.Error(fmt.Sprintf("[%s] index check error: %v", target.Name, err))
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
	rootCmd.AddCommand(indexCmd)
}
