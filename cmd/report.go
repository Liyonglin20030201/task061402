package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Liyonglin20030201/task061402/internal/report"
)

var (
	reportFormat string
	reportOutput string
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate inspection report",
	Long:  "Export inspection results as HTML, JSON, or CSV report from the most recent run.",
	RunE: func(cmd *cobra.Command, args []string) error {
		format := reportFormat
		if format == "" {
			format = cfg.Report.Format
		}

		outputDir := reportOutput
		if outputDir == "" {
			outputDir = cfg.Global.ReportDir
		}

		inspections, err := db.GetInspectionsByRunID(runID)
		if err != nil {
			return fmt.Errorf("failed to load inspection results: %w", err)
		}

		if len(inspections) == 0 {
			fmt.Println("No inspection results found for current run.")
			fmt.Println("Run 'dbinspect inspect' first, then generate a report.")
			return nil
		}

		gen, err := report.NewGenerator(format)
		if err != nil {
			return err
		}

		filePath, err := gen.Generate(inspections, runID, outputDir)
		if err != nil {
			return fmt.Errorf("report generation failed: %w", err)
		}

		info, _ := os.Stat(filePath)
		var fileSize int64
		if info != nil {
			fileSize = info.Size()
		}

		db.SaveReport(runID, format, filePath, fileSize)

		fmt.Printf("Report generated: %s\n", filePath)
		fmt.Printf("Format: %s | Size: %d bytes\n", format, fileSize)

		return nil
	},
}

func init() {
	reportCmd.Flags().StringVar(&reportFormat, "format", "", "report format: html|json|csv")
	reportCmd.Flags().StringVarP(&reportOutput, "output", "o", "", "output directory")
	rootCmd.AddCommand(reportCmd)
}
