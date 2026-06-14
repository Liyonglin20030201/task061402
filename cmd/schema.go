package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Liyonglin20030201/task061402/internal/connector"
	"github.com/Liyonglin20030201/task061402/internal/inspector"
	"github.com/Liyonglin20030201/task061402/internal/store"
)

var snapshotOnly bool

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Detect schema changes",
	Long:  "Compare current database schema against previous snapshot to detect structural changes (tables, columns, indexes).",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets := resolveTargets()
		insp := inspector.NewSchemaChangeInspector(db)

		for _, target := range targets {
			if globalCtx.Err() != nil {
				fmt.Printf("  ⚠ interrupted, stopping further checks\n")
				break
			}

			if target.Type == "redis" {
				fmt.Printf("  - %s: schema check not applicable to Redis\n", target.Name)
				continue
			}

			conn, err := connector.NewFromTarget(target)
			if err != nil {
				log.Error(fmt.Sprintf("[%s] connector error: %v", target.Name, err))
				continue
			}

			if err := connectWithRetry(globalCtx, conn, target); err != nil {
				log.Error(fmt.Sprintf("[%s] %v", target.Name, err))
				fmt.Printf("  ✗ %s: %v\n", target.Name, err)
				conn.Close()
				continue
			}

			checkCtx, checkCancel := createCheckContext(globalCtx, cfg.Global.Timeout)
			result, err := insp.Run(checkCtx, conn, cfg)
			checkCancel()
			conn.Close()

			if err != nil {
				log.Error(fmt.Sprintf("[%s] schema check error: %v", target.Name, err))
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
				if changes, ok := result.Details["total_changes"]; ok {
					fmt.Printf("    Total changes: %v\n", changes)
				}
			}
		}

		fmt.Printf("\nRun ID: %s\n", runID)
		return nil
	},
}

func init() {
	schemaCmd.Flags().BoolVar(&snapshotOnly, "snapshot-only", false, "only take a snapshot without comparing")
	rootCmd.AddCommand(schemaCmd)
}
