package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/Liyonglin20030201/task061402/internal/trend"
)

var (
	trendSince  string
	trendUntil  string
	trendCheck  string
	trendTarget string
)

var trendCmd = &cobra.Command{
	Use:   "trend",
	Short: "Analyze historical trends",
	Long:  "Compare historical inspection data to detect trends in risk scores, capacity, and other metrics.",
	RunE: func(cmd *cobra.Command, args []string) error {
		analyzer := trend.NewAnalyzer(db)

		end := time.Now()
		if trendUntil != "" {
			d, err := time.ParseDuration(trendUntil)
			if err != nil {
				t, err2 := time.Parse("2006-01-02", trendUntil)
				if err2 != nil {
					return fmt.Errorf("invalid --until value %q: use duration (e.g. 24h) or date (2006-01-02)", trendUntil)
				}
				end = t.Add(24*time.Hour - time.Second)
			} else {
				end = time.Now().Add(-d)
			}
		}

		start := end.Add(-7 * 24 * time.Hour)
		if trendSince != "" {
			d, err := time.ParseDuration(trendSince)
			if err != nil {
				t, err2 := time.Parse("2006-01-02", trendSince)
				if err2 != nil {
					return fmt.Errorf("invalid --since value %q: use duration (e.g. 168h) or date (2006-01-02)", trendSince)
				}
				start = t
			} else {
				start = time.Now().Add(-d)
			}
		}

		tr := trend.TimeRange{Start: start, End: end}

		targets := []string{trendTarget}
		if trendTarget == "" {
			var err error
			targets, err = db.GetAllTargetNames()
			if err != nil {
				return fmt.Errorf("failed to get targets: %w", err)
			}
		}

		if len(targets) == 0 {
			fmt.Println("No inspection data found. Run 'dbinspect inspect' first.")
			return nil
		}

		fmt.Printf("━━━ Trend Analysis ━━━\n")
		fmt.Printf("  Period: %s to %s\n\n", start.Format("2006-01-02 15:04"), end.Format("2006-01-02 15:04"))

		for _, target := range targets {
			if target == "" {
				continue
			}
			fmt.Printf("  [%s]\n", target)

			checkTypes := []string{trendCheck}
			if trendCheck == "" {
				var err error
				checkTypes, err = db.GetCheckTypesForTarget(target)
				if err != nil {
					log.Warn(fmt.Sprintf("failed to get check types for %s: %v", target, err))
					continue
				}
			}

			for _, ct := range checkTypes {
				if ct == "" {
					continue
				}
				result, err := analyzer.DetectTrend(target, ct, tr, 100)
				if err != nil {
					log.Warn(fmt.Sprintf("[%s] %s trend error: %v", target, ct, err))
					continue
				}

				arrow := "→"
				switch result.Direction {
				case "degrading":
					arrow = "↑"
				case "improving":
					arrow = "↓"
				}

				fmt.Printf("    %s %s: %s (%.1f%%, %d data points)\n",
					arrow, ct, result.Direction, result.DeltaPct, len(result.DataPoints))
			}
			fmt.Println()
		}

		fmt.Printf("Run ID: %s\n", runID)
		return nil
	},
}

func init() {
	trendCmd.Flags().StringVar(&trendSince, "since", "", "start of analysis period (duration like '168h' or date like '2006-01-02')")
	trendCmd.Flags().StringVar(&trendUntil, "until", "", "end of analysis period (duration or date)")
	trendCmd.Flags().StringVar(&trendCheck, "check", "", "filter by check type (e.g. 'capacity', 'slowquery')")
	trendCmd.Flags().StringVar(&trendTarget, "target", "", "filter by target name")
	rootCmd.AddCommand(trendCmd)
}
