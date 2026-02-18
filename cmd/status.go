package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/larspittman/cash/internal/db"
	"github.com/larspittman/cash/internal/models"
	"github.com/larspittman/cash/internal/report"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show active timer and recent activity summary",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		// Active timer
		timer, err := d.GetActiveTimer()
		if err != nil {
			return err
		}
		if timer != nil {
			elapsed := time.Since(timer.StartedAt)
			fmt.Printf("● Timer running  →  %s\n", timer.ProjectName)
			fmt.Printf("  Started : %s  (%s ago)\n",
				timer.StartedAt.Format("15:04"), formatDuration(elapsed))
			if timer.Message != "" {
				fmt.Printf("  Note    : %s\n", timer.Message)
			}
		} else {
			fmt.Println("○ No active timer")
		}

		fmt.Println()

		// This week summary
		now := time.Now()
		ws := report.WeekStart(now)
		we := ws.AddDate(0, 0, 7)
		weekEntries, err := d.GetTimeEntries(models.TimeEntryFilter{From: &ws, To: &we})
		if err != nil {
			return err
		}
		weekHours := 0.0
		for _, e := range weekEntries {
			weekHours += e.Hours
		}

		// This month summary
		ms := report.MonthStart(now)
		me := report.MonthEnd(now)
		monthEntries, err := d.GetTimeEntries(models.TimeEntryFilter{From: &ms, To: &me})
		if err != nil {
			return err
		}
		monthHours := 0.0
		for _, e := range monthEntries {
			monthHours += e.Hours
		}

		fmt.Printf("This week  : %.2fh\n", weekHours)
		fmt.Printf("This month : %.2fh\n", monthHours)

		// Recent 5 entries
		allEntries, err := d.GetTimeEntries(models.TimeEntryFilter{})
		if err != nil {
			return err
		}
		if len(allEntries) > 0 {
			fmt.Println("\nRecent entries:")
			start := len(allEntries) - 5
			if start < 0 {
				start = 0
			}
			recent := allEntries[start:]
			for i := len(recent) - 1; i >= 0; i-- {
				e := recent[i]
				msg := e.Message
				if msg == "" {
					msg = "-"
				}
				fmt.Printf("  %s  %-15s  %5.2fh  %s\n",
					e.CommittedAt.Format("2006-01-02"),
					e.ProjectName,
					e.Hours,
					msg,
				)
			}
		}

		return nil
	},
}
