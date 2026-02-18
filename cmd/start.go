package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/larspittman/cash/internal/db"
)

var startMsg string

var startCmd = &cobra.Command{
	Use:   "start <project>",
	Short: "Start a timer for a project",
	Long: `Begin timing work on a project. Use 'cash stop' to commit the hours.

Examples:
  cash start web_app -m "Working on dashboard"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]

		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		// Warn if a timer is already running
		existing, err := d.GetActiveTimer()
		if err != nil {
			return err
		}
		if existing != nil {
			elapsed := time.Since(existing.StartedAt)
			fmt.Printf("Warning: timer already running for %q (started %s ago)\n",
				existing.ProjectName, formatDuration(elapsed))
			fmt.Println("Run 'cash stop' first to commit those hours.")
			return nil
		}

		if _, err := d.StartTimer(project, startMsg); err != nil {
			return fmt.Errorf("starting timer: %w", err)
		}

		fmt.Printf("Timer started  →  %s  [%s]\n", project, time.Now().Format("15:04"))
		if startMsg != "" {
			fmt.Printf("  %s\n", startMsg)
		}
		return nil
	},
}

func init() {
	startCmd.Flags().StringVarP(&startMsg, "message", "m", "", "What you're working on")
}

// formatDuration returns a human-readable duration like "2h 15m".
func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
