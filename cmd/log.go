package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/larspittman/cash/internal/db"
	"github.com/larspittman/cash/internal/models"
	"github.com/larspittman/cash/internal/report"
)

var (
	logProject string
	logWeek    bool
	logMonth   bool
	logAll     bool
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Show time entry log",
	Long: `Display committed time entries grouped by day.

Defaults to the current week. Use flags to change scope.

Examples:
  cash log
  cash log --month
  cash log -p web_app
  cash log --all`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		now := time.Now()
		f := models.TimeEntryFilter{ProjectName: logProject}

		switch {
		case logAll:
			// no date filter
		case logMonth:
			ms := report.MonthStart(now)
			me := report.MonthEnd(now)
			f.From, f.To = &ms, &me
		default: // default: current week
			ws := report.WeekStart(now)
			we := ws.AddDate(0, 0, 7)
			f.From, f.To = &ws, &we
			if logWeek {
				// explicit --week flag, same behaviour
			}
		}

		entries, err := d.GetTimeEntries(f)
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			fmt.Println("No entries found.")
			return nil
		}

		// Group by date
		type dayGroup struct {
			date    time.Time
			entries []models.TimeEntry
		}
		var groups []dayGroup
		groupIdx := map[string]int{}

		for _, e := range entries {
			key := e.CommittedAt.Format("2006-01-02")
			if i, ok := groupIdx[key]; ok {
				groups[i].entries = append(groups[i].entries, e)
			} else {
				groupIdx[key] = len(groups)
				groups = append(groups, dayGroup{
					date:    e.CommittedAt.Truncate(24 * time.Hour),
					entries: []models.TimeEntry{e},
				})
			}
		}

		today := now.Truncate(24 * time.Hour)
		yesterday := today.AddDate(0, 0, -1)

		// Print newest first
		for i := len(groups) - 1; i >= 0; i-- {
			g := groups[i]
			label := g.date.Format("2006-01-02  Mon")
			switch {
			case g.date.Equal(today):
				label += "  (today)"
			case g.date.Equal(yesterday):
				label += "  (yesterday)"
			}

			dayTotal := 0.0
			for _, e := range g.entries {
				dayTotal += e.Hours
			}
			fmt.Printf("■ %s  [%.2fh]\n", label, dayTotal)

			for _, e := range g.entries {
				sub := ""
				if e.Subservice != "" {
					sub = "  (" + e.Subservice + ")"
				}
				msg := e.Message
				if msg == "" {
					msg = "-"
				}
				timeStr := e.CommittedAt.Format("15:04")
				if e.StartTime != nil && e.EndTime != nil {
					timeStr = fmt.Sprintf("%s–%s", e.StartTime.Format("15:04"), e.EndTime.Format("15:04"))
				}
				fmt.Printf("  %s  %-16s  %5.2fh%s  %s\n",
					timeStr, e.ProjectName, e.Hours, sub, msg)
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	logCmd.Flags().StringVarP(&logProject, "project", "p", "", "Filter by project name")
	logCmd.Flags().BoolVarP(&logWeek, "week", "w", false, "Show current week (default)")
	logCmd.Flags().BoolVarP(&logMonth, "month", "m", false, "Show current month")
	logCmd.Flags().BoolVarP(&logAll, "all", "a", false, "Show all entries")
}
