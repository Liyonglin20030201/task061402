package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/Liyonglin20030201/task061402/internal/connector"
	"github.com/Liyonglin20030201/task061402/internal/inspector"
	"github.com/Liyonglin20030201/task061402/internal/store"
)

var pingCmd = &cobra.Command{
	Use:   "ping",
	Short: "Test database connectivity",
	Long:  "Verify that the specified database targets are reachable and responding to connections.",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets := resolveTargets()
		ping := inspector.NewPingInspector()

		var hasError bool
		for _, target := range targets {
			if globalCtx.Err() != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "  ⚠ interrupted, stopping further checks\n")
				break
			}

			conn, err := connector.NewFromTarget(target)
			if err != nil {
				log.Error(fmt.Sprintf("[%s] connector creation failed: %v", target.Name, err))
				fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %s (%s): FAILED - %v\n", target.Name, target.Type, err)
				hasError = true
				continue
			}

			if err := connectWithRetry(globalCtx, conn, target); err != nil {
				log.Error(fmt.Sprintf("[%s] %v", target.Name, err))
				fmt.Printf("  ✗ %s (%s): FAILED - %v\n", target.Name, target.Type, err)

				db.SaveInspection(&store.Inspection{
					RunID:      runID,
					TargetName: target.Name,
					TargetType: target.Type,
					CheckType:  "ping",
					Status:     string(inspector.StatusError),
					RiskScore:  100,
					Summary:    fmt.Sprintf("connection failed: %v", err),
					StartedAt:  time.Now(),
					FinishedAt: time.Now(),
				})

				hasError = true
				continue
			}

			// 单次 ping 也绑定超时 context
			pingCtx, pingCancel := createCheckContext(globalCtx, cfg.Global.Timeout)
			result, _ := ping.Run(pingCtx, conn, cfg)
			pingCancel()
			conn.Close()

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

			switch result.Status {
			case inspector.StatusSuccess:
				fmt.Printf("  ✓ %s (%s): %s\n", target.Name, target.Type, result.Summary)
			case inspector.StatusWarning:
				fmt.Printf("  ⚠ %s (%s): %s\n", target.Name, target.Type, result.Summary)
			default:
				fmt.Printf("  ✗ %s (%s): %s\n", target.Name, target.Type, result.Summary)
				hasError = true
			}
		}

		if hasError {
			return fmt.Errorf("one or more targets failed connectivity check")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pingCmd)
}
