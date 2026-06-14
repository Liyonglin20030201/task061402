package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("dbinspect %s (built %s)\n", Version, BuildTime)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
