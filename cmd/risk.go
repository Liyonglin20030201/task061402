package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Liyonglin20030201/task061402/internal/connector"
	"github.com/Liyonglin20030201/task061402/internal/inspector"
	"github.com/Liyonglin20030201/task061402/internal/store"
)

var riskCmd = &cobra.Command{
	Use:   "risk",
	Short: "Calculate risk score",
	Long:  "Run all inspections and compute a weighted risk score for the target databases.",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets := resolveTargets()

		inspectors := []inspector.Inspector{
			inspector.NewPingInspector(),
			inspector.NewSlowQueryInspector(),
			inspector.NewCapacityInspector(),
			inspector.NewIndexInspector(),
			inspector.NewBackupInspector(),
			inspector.NewPermissionInspector(),
			inspector.NewReplicationInspector(),
			inspector.NewSchemaChangeInspector(db),
		}

		for _, target := range targets {
			if globalCtx.Err() != nil {
				fmt.Printf("\n  ⚠ interrupted, stopping further targets\n")
				break
			}

			fmt.Printf("\n[%s] (%s) Running risk assessment...\n", target.Name, target.Type)

			conn, err := connector.NewFromTarget(target)
			if err != nil {
				log.Error(fmt.Sprintf("[%s] connector error: %v", target.Name, err))
				continue
			}

			if err := connectWithRetry(globalCtx, conn, target); err != nil {
				log.Error(fmt.Sprintf("[%s] %v", target.Name, err))
				fmt.Printf("  ✗ %v\n", err)
				continue
			}

			var results []*inspector.Result
			for _, insp := range inspectors {
				if globalCtx.Err() != nil {
					fmt.Printf("  ⚠ interrupted\n")
					break
				}

				checkCtx, checkCancel := createCheckContext(globalCtx, cfg.Global.Timeout)
				result, err := insp.Run(checkCtx, conn, cfg)
				checkCancel()

				if err != nil {
					log.Warn(fmt.Sprintf("[%s] %s check error: %v", target.Name, insp.Name(), err))
					continue
				}
				results = append(results, result)
				printResult(target.Name, result)

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
			}

			conn.Close()

			score, categoryScores := inspector.ComputeRiskScore(results, cfg.Risk.Weights)
			fmt.Printf("\n  %s\n", inspector.RiskSummary(score))
			if verbose {
				for cat, s := range categoryScores {
					fmt.Printf("    %s: %d/100\n", cat, s)
				}
			}
		}

		fmt.Printf("\nRun ID: %s\n", runID)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(riskCmd)
}
