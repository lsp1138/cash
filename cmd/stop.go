package cmd

import (
	"fmt"
	"math"
	"time"

	"github.com/spf13/cobra"

	"github.com/larspittman/cash/internal/db"
	"github.com/larspittman/cash/internal/models"
)

var stopMsg string

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the active timer and commit hours",
	Long: `Stop the running timer, calculate elapsed hours, and save the time entry.

Examples:
  cash stop
  cash stop -m "Finished login flow"`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		timer, err := d.GetActiveTimer()
		if err != nil {
			return err
		}
		if timer == nil {
			fmt.Println("No active timer. Use 'cash start <project>' to begin.")
			return nil
		}

		now := time.Now()
		elapsed := now.Sub(timer.StartedAt)
		hours := roundHours(elapsed)

		message := timer.Message
		if stopMsg != "" {
			if message != "" {
				message = message + " / " + stopMsg
			} else {
				message = stopMsg
			}
		}

		entry := models.TimeEntry{
			ProjectName: timer.ProjectName,
			Hours:       hours,
			Message:     message,
			StartTime:   &timer.StartedAt,
			EndTime:     &now,
			CommittedAt: now,
		}
		if _, err := d.AddTimeEntry(entry); err != nil {
			return fmt.Errorf("saving entry: %w", err)
		}
		if err := d.DeleteTimer(timer.ID); err != nil {
			return fmt.Errorf("clearing timer: %w", err)
		}

		fmt.Printf("Timer stopped  →  %s\n", timer.ProjectName)
		fmt.Printf("  Started : %s\n", timer.StartedAt.Format("15:04"))
		fmt.Printf("  Stopped : %s\n", now.Format("15:04"))
		fmt.Printf("  Elapsed : %s  →  %.2fh committed\n", formatDuration(elapsed), hours)
		if message != "" {
			fmt.Printf("  Note    : %s\n", message)
		}
		return nil
	},
}

func init() {
	stopCmd.Flags().StringVarP(&stopMsg, "message", "m", "", "Final note to append to the entry")
}

// roundHours rounds duration to the nearest 6-minute block (0.1h precision).
func roundHours(d time.Duration) float64 {
	minutes := d.Minutes()
	return math.Round(minutes/6) * 6 / 60
}
