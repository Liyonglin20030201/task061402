package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Liyonglin20030201/task061402/internal/connector"
	"github.com/Liyonglin20030201/task061402/internal/inspector"
	"github.com/Liyonglin20030201/task061402/internal/report"
	"github.com/Liyonglin20030201/task061402/internal/store"
)

var (
	inspectReport bool
	inspectFormat string
)

var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Run full database inspection",
	Long:  "Execute all inspection checks against target databases and optionally generate a report.",
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

		var allResults []*inspector.Result

		for _, target := range targets {
			fmt.Printf("\n━━━ %s (%s) ━━━\n", target.Name, target.Type)

			ctx, cancel := context.WithTimeout(context.Background(), cfg.Global.Timeout*3)

			conn, err := connector.NewFromTarget(target)
			if err != nil {
				log.Error(fmt.Sprintf("[%s] connector error: %v", target.Name, err))
				fmt.Printf("  ✗ Failed to create connector: %v\n", err)
				cancel()
				continue
			}

			if err := conn.Connect(ctx); err != nil {
				log.Error(fmt.Sprintf("[%s] connection failed: %v", target.Name, err))
				fmt.Printf("  ✗ Connection failed: %v\n", err)
				cancel()
				continue
			}

			for _, insp := range inspectors {
				select {
				case <-ctx.Done():
					log.Warn(fmt.Sprintf("[%s] inspection cancelled: timeout", target.Name))
					fmt.Printf("  ⚠ Inspection timeout - partial results saved\n")
					goto nextTarget
				default:
				}

				result, err := insp.Run(ctx, conn, cfg)
				if err != nil {
					log.Warn(fmt.Sprintf("[%s] %s error: %v", target.Name, insp.Name(), err))
					continue
				}

				allResults = append(allResults, result)
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

		nextTarget:
			conn.Close()
			cancel()
		}

		// Risk scoring
		score, _ := inspector.ComputeRiskScore(allResults, cfg.Risk.Weights)
		fmt.Printf("\n━━━ Summary ━━━\n")
		fmt.Printf("  %s\n", inspector.RiskSummary(score))
		fmt.Printf("  Checks run: %d\n", len(allResults))
		fmt.Printf("  Run ID: %s\n", runID)

		// Generate report if requested
		if inspectReport {
			format := inspectFormat
			if format == "" {
				format = cfg.Report.Format
			}

			inspections, _ := db.GetInspectionsByRunID(runID)
			gen, err := report.NewGenerator(format)
			if err != nil {
				return err
			}

			filePath, err := gen.Generate(inspections, runID, cfg.Global.ReportDir)
			if err != nil {
				log.Error(fmt.Sprintf("report generation failed: %v", err))
				fmt.Printf("  ⚠ Report generation failed: %v\n", err)
			} else {
				info, _ := os.Stat(filePath)
				var fileSize int64
				if info != nil {
					fileSize = info.Size()
				}
				db.SaveReport(runID, format, filePath, fileSize)
				fmt.Printf("  Report: %s\n", filePath)
			}
		}

		// Exit code based on risk
		if score >= 80 {
			os.Exit(2)
		} else if score >= 40 {
			os.Exit(1)
		}

		return nil
	},
}

func init() {
	inspectCmd.Flags().BoolVar(&inspectReport, "report", false, "generate report after inspection")
	inspectCmd.Flags().StringVar(&inspectFormat, "format", "", "report format: html|json|csv")
	rootCmd.AddCommand(inspectCmd)
}
