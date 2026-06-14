package cmd

import (
	"context"
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
		}

		for _, target := range targets {
			fmt.Printf("\n[%s] (%s) Running risk assessment...\n", target.Name, target.Type)

			ctx, cancel := context.WithTimeout(context.Background(), cfg.Global.Timeout*2)

			conn, err := connector.NewFromTarget(target)
			if err != nil {
				log.Error(fmt.Sprintf("[%s] connector error: %v", target.Name, err))
				cancel()
				continue
			}

			if err := conn.Connect(ctx); err != nil {
				log.Error(fmt.Sprintf("[%s] connection failed: %v", target.Name, err))
				fmt.Printf("  ✗ connection failed: %v\n", err)
				cancel()
				continue
			}

			var results []*inspector.Result
			for _, insp := range inspectors {
				result, err := insp.Run(ctx, conn, cfg)
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
			cancel()

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
