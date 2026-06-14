package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Liyonglin20030201/task061402/internal/connector"
	"github.com/Liyonglin20030201/task061402/internal/inspector"
	"github.com/Liyonglin20030201/task061402/internal/store"
)

var permissionCmd = &cobra.Command{
	Use:   "permission",
	Short: "Scan database permissions",
	Long:  "Audit user privileges and identify potentially dangerous permission configurations.",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets := resolveTargets()
		insp := inspector.NewPermissionInspector()

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
				log.Error(fmt.Sprintf("[%s] permission check error: %v", target.Name, err))
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
	rootCmd.AddCommand(permissionCmd)
}
