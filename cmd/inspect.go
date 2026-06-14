package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

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
		// 跟踪所有打开的连接，确保巡检结束时统一关闭
		var openConns []connector.Connector

		defer func() {
			for _, c := range openConns {
				c.Close()
			}
		}()

		for _, target := range targets {
			if globalCtx.Err() != nil {
				fmt.Printf("\n  ⚠ interrupted, stopping further targets\n")
				break
			}

			fmt.Printf("\n━━━ %s (%s) ━━━\n", target.Name, target.Type)

			conn, err := connector.NewFromTarget(target)
			if err != nil {
				log.Error(fmt.Sprintf("[%s] connector error: %v", target.Name, err))
				fmt.Printf("  ✗ Failed to create connector: %v\n", err)

				// 记录错误结果，继续下一个 target（不退出）
				db.SaveInspection(&store.Inspection{
					RunID:      runID,
					TargetName: target.Name,
					TargetType: target.Type,
					CheckType:  "ping",
					Status:     string(inspector.StatusError),
					RiskScore:  100,
					Summary:    fmt.Sprintf("connector creation failed: %v", err),
					StartedAt:  time.Now(),
					FinishedAt: time.Now(),
				})
				continue
			}
			openConns = append(openConns, conn)

			if err := connectWithRetry(globalCtx, conn, target); err != nil {
				log.Error(fmt.Sprintf("[%s] %v", target.Name, err))
				fmt.Printf("  ✗ %v\n", err)

				// 权限不足或连接失败：记录错误结果，继续下一个 target（不退出）
				allResults = append(allResults, &inspector.Result{
					CheckType:  "ping",
					Status:     inspector.StatusError,
					RiskScore:  100,
					Summary:    fmt.Sprintf("connection failed: %v", err),
					StartedAt:  time.Now(),
					FinishedAt: time.Now(),
				})
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
				// 连接失败则关闭并从跟踪列表移除（释放连接池资源）
				conn.Close()
				openConns = openConns[:len(openConns)-1]
				continue
			}

			for _, insp := range inspectors {
				if globalCtx.Err() != nil {
					log.Warn(fmt.Sprintf("[%s] inspection cancelled: interrupted", target.Name))
					fmt.Printf("  ⚠ Interrupted - partial results saved\n")
					break
				}

				checkCtx, checkCancel := createCheckContext(globalCtx, cfg.Global.Timeout)
				result, err := insp.Run(checkCtx, conn, cfg)
				checkCancel()

				if err != nil {
					// 检查执行出错（如权限不足）：记录警告并跳过，继续后续巡检项
					log.Warn(fmt.Sprintf("[%s] %s error: %v", target.Name, insp.Name(), err))
					errResult := inspector.NewResult(insp.Name())
					errResult.RiskScore = 20
					errResult.Details["error"] = err.Error()
					errResult.Finish(inspector.StatusWarning,
						fmt.Sprintf("%s skipped due to error: %v", insp.Name(), err))
					allResults = append(allResults, errResult)
					printResult(target.Name, errResult)
					db.SaveInspection(&store.Inspection{
						RunID:      runID,
						TargetName: target.Name,
						TargetType: target.Type,
						CheckType:  errResult.CheckType,
						Status:     string(errResult.Status),
						RiskScore:  errResult.RiskScore,
						Summary:    errResult.Summary,
						Details:    errResult.Details,
						StartedAt:  errResult.StartedAt,
						FinishedAt: errResult.FinishedAt,
					})
					continue
				}

				// 如果检查超时，标注并强制回收连接池中的死连接
				if checkCtx.Err() == context.DeadlineExceeded {
					if result.Status != inspector.StatusWarning {
						result.Status = inspector.StatusWarning
						result.Summary = fmt.Sprintf("%s (timed out after %s)", result.Summary, cfg.Global.Timeout)
					}
					result.Details["timeout"] = true
					log.Warn(fmt.Sprintf("[%s] %s timed out after %s, resetting connection pool", target.Name, insp.Name(), cfg.Global.Timeout))

					// 超时后强制关闭当前连接，并重新建立连接
					// 这防止连接池中半死连接占用资源影响后续检查
					conn.Close()
					openConns = openConns[:len(openConns)-1]

					newConn, reconnErr := connector.NewFromTarget(target)
					if reconnErr == nil {
						reconnCtx, reconnCancel := createCheckContext(globalCtx, cfg.Global.Timeout)
						if connErr := newConn.Connect(reconnCtx); connErr == nil {
							conn = newConn
							openConns = append(openConns, conn)
						} else {
							log.Warn(fmt.Sprintf("[%s] reconnection failed after timeout: %v, skipping remaining checks", target.Name, connErr))
							newConn.Close()
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
							reconnCancel()
							break
						}
						reconnCancel()
					}
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
