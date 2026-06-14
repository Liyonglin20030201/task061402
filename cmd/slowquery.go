package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Liyonglin20030201/task061402/internal/connector"
	"github.com/Liyonglin20030201/task061402/internal/inspector"
	"github.com/Liyonglin20030201/task061402/internal/store"
)

var slowqueryTopN int

var slowqueryCmd = &cobra.Command{
	Use:   "slowquery",
	Short: "Analyze slow queries",
	Long:  "Analyze slow queries from performance_schema (MySQL), pg_stat_statements (PostgreSQL), or slowlog (Redis).",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets := resolveTargets()
		insp := inspector.NewSlowQueryInspector()

		if slowqueryTopN > 0 {
			cfg.Checks.SlowQuery.TopN = slowqueryTopN
		}

		for _, target := range targets {
			ctx, cancel := context.WithTimeout(context.Background(), cfg.Global.Timeout)

			conn, err := connector.NewFromTarget(target)
			if err != nil {
				log.Error(fmt.Sprintf("[%s] connector error: %v", target.Name, err))
				cancel()
				continue
			}

			if err := conn.Connect(ctx); err != nil {
				log.Error(fmt.Sprintf("[%s] connection failed: %v", target.Name, err))
				fmt.Printf("  ✗ %s: connection failed - %v\n", target.Name, err)
				cancel()
				continue
			}

			result, err := insp.Run(ctx, conn, cfg)
			conn.Close()
			cancel()

			if err != nil {
				log.Error(fmt.Sprintf("[%s] slow query check error: %v", target.Name, err))
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
				if count, ok := result.Details["count"]; ok {
					fmt.Printf("    Slow queries found: %v\n", count)
				}
			}
		}

		fmt.Printf("\nRun ID: %s\n", runID)
		return nil
	},
}

func init() {
	slowqueryCmd.Flags().IntVar(&slowqueryTopN, "top", 0, "number of top slow queries to show")
	rootCmd.AddCommand(slowqueryCmd)
}

func printResult(targetName string, result *inspector.Result) {
	switch result.Status {
	case inspector.StatusSuccess:
		fmt.Printf("  ✓ %s [%s]: %s\n", targetName, result.CheckType, result.Summary)
	case inspector.StatusWarning:
		fmt.Printf("  ⚠ %s [%s]: %s\n", targetName, result.CheckType, result.Summary)
	case inspector.StatusError:
		fmt.Printf("  ✗ %s [%s]: %s\n", targetName, result.CheckType, result.Summary)
	case inspector.StatusSkipped:
		fmt.Printf("  - %s [%s]: %s\n", targetName, result.CheckType, result.Summary)
	}
}
