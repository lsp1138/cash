package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/larspittman/cash/internal/db"
	"github.com/larspittman/cash/internal/models"
	"github.com/larspittman/cash/internal/report"
)

var weekDate string

var weekCmd = &cobra.Command{
	Use:   "week",
	Short: "Show a weekly calendar of time entries",
	Long: `Display a grid of hours per project per day for a given week.

Examples:
  cash week
  cash week --date 2026-02-10`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		target := time.Now()
		if weekDate != "" {
			var err error
			target, err = time.Parse("2006-01-02", weekDate)
			if err != nil {
				return fmt.Errorf("invalid date %q: use YYYY-MM-DD", weekDate)
			}
		}

		ws := report.WeekStart(target)
		we := ws.AddDate(0, 0, 7)

		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		entries, err := d.GetTimeEntries(models.TimeEntryFilter{From: &ws, To: &we})
		if err != nil {
			return err
		}

		printWeekCalendar(entries, ws)
		return nil
	},
}

func init() {
	weekCmd.Flags().StringVar(&weekDate, "date", "", "Any date within the target week (YYYY-MM-DD)")
}

func printWeekCalendar(entries []models.TimeEntry, weekStart time.Time) {
	dayNames := [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	days := [7]time.Time{}
	for i := range days {
		days[i] = weekStart.AddDate(0, 0, i)
	}

	// Collect project hours per day
	type row struct{ hours [7]float64 }
	projectRows := map[string]*row{}
	projectOrder := []string{}

	for _, e := range entries {
		idx := report.DayIndex(e.CommittedAt.Weekday())
		if _, exists := projectRows[e.ProjectName]; !exists {
			projectRows[e.ProjectName] = &row{}
			projectOrder = append(projectOrder, e.ProjectName)
		}
		projectRows[e.ProjectName].hours[idx] += e.Hours
	}

	// Day totals
	var dayTotals [7]float64
	for _, r := range projectRows {
		for i, h := range r.hours {
			dayTotals[i] += h
		}
	}
	grandTotal := 0.0
	for _, h := range dayTotals {
		grandTotal += h
	}

	// Column widths
	nameW := 14
	for _, name := range projectOrder {
		if len(name) > nameW {
			nameW = len(name)
		}
	}
	if nameW > 22 {
		nameW = 22
	}

	week, year := weekStart.ISOWeek()
	fmt.Printf("Week %02d · %d   %s – %s\n\n",
		week, year,
		weekStart.Format("Mon Jan 2"),
		weekStart.AddDate(0, 0, 6).Format("Mon Jan 2"),
	)

	sep := func() {
		fmt.Print(strings.Repeat("─", nameW+1))
		fmt.Print("┼")
		for range dayNames {
			fmt.Print("────────┼")
		}
		fmt.Println("────────")
	}

	// Header row
	fmt.Printf("%-*s │", nameW, "Project")
	for i, dn := range dayNames {
		fmt.Printf(" %s %2d │", dn, days[i].Day())
	}
	fmt.Println("   Total")
	sep()

	// Project rows
	for _, name := range projectOrder {
		r := projectRows[name]
		display := name
		if len(display) > nameW {
			display = display[:nameW-1] + "…"
		}
		fmt.Printf("%-*s │", nameW, display)
		rowTotal := 0.0
		for _, h := range r.hours {
			if h > 0 {
				fmt.Printf("  %4.1f  │", h)
			} else {
				fmt.Printf("        │")
			}
			rowTotal += h
		}
		fmt.Printf("   %4.1f\n", rowTotal)
	}

	if len(projectOrder) == 0 {
		fmt.Printf("%-*s │", nameW, "(no entries)")
		for range dayNames {
			fmt.Print("        │")
		}
		fmt.Println("    0.0")
	}

	sep()

	// Total row
	fmt.Printf("%-*s │", nameW, "Total")
	for _, h := range dayTotals {
		if h > 0 {
			fmt.Printf("  %4.1f  │", h)
		} else {
			fmt.Printf("   0.0  │")
		}
	}
	fmt.Printf("   %4.1f\n", grandTotal)
}
