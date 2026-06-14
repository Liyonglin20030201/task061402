package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/Liyonglin20030201/task061402/internal/notifier"
)

var notifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Test notification channels",
	Long:  "Send a test notification to all configured channels to verify they are working correctly.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !cfg.Notifications.Enabled {
			fmt.Println("Notifications are not enabled in config.")
			return nil
		}

		channels := buildNotificationChannels(cfg)
		if len(channels) == 0 {
			fmt.Println("No notification channels configured.")
			return nil
		}

		event := &notifier.AlertEvent{
			TargetName: "test",
			CheckType:  "test",
			RiskScore:  75,
			RiskLevel:  "HIGH",
			Summary:    "This is a test notification from dbinspect",
			OccurredAt: time.Now(),
			RunID:      runID,
		}

		fmt.Printf("Sending test notification to %d channel(s)...\n", len(channels))

		for _, ch := range channels {
			if err := ch.Send(globalCtx, event); err != nil {
				fmt.Printf("  ✗ %s: %v\n", ch.Name(), err)
			} else {
				fmt.Printf("  ✓ %s: sent successfully\n", ch.Name())
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(notifyCmd)
}
