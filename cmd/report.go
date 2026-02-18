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

var (
	reportDate    string
	reportProject string
)

var reportCmd = &cobra.Command{
	Use:   "report <week|month>",
	Short: "Generate a time and revenue report",
	Long: `Show hours and revenue breakdown.

Examples:
  cash report week
  cash report month
  cash report week --date 2026-02-10
  cash report month --date 2026-01
  cash report month -p web_app`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"week", "month"},
	RunE: func(cmd *cobra.Command, args []string) error {
		period := args[0]
		if period != "week" && period != "month" {
			return fmt.Errorf("unknown period %q: use 'week' or 'month'", period)
		}

		target := time.Now()
		if reportDate != "" {
			var err error
			// Try full date first, then year-month
			target, err = time.Parse("2006-01-02", reportDate)
			if err != nil {
				target, err = time.Parse("2006-01", reportDate)
				if err != nil {
					return fmt.Errorf("invalid date %q: use YYYY-MM-DD or YYYY-MM", reportDate)
				}
			}
		}

		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		var from, to time.Time
		if period == "week" {
			from = report.WeekStart(target)
			to = from.AddDate(0, 0, 7)
		} else {
			from = report.MonthStart(target)
			to = report.MonthEnd(target)
		}

		f := models.TimeEntryFilter{From: &from, To: &to, ProjectName: reportProject}
		entries, err := d.GetTimeEntries(f)
		if err != nil {
			return err
		}

		// Build project map for rate lookups
		projects, err := d.GetProjects()
		if err != nil {
			return err
		}
		pm := make(map[string]*models.Project, len(projects))
		for i := range projects {
			p := projects[i]
			pm[p.Name] = &p
		}

		if period == "week" {
			wd := report.BuildWeek(entries, pm, from)
			printWeekReport(wd)
		} else {
			md := report.BuildMonth(entries, pm, from)
			printMonthReport(md)
		}
		return nil
	},
}

func init() {
	reportCmd.Flags().StringVar(&reportDate, "date", "", "Target date (YYYY-MM-DD or YYYY-MM)")
	reportCmd.Flags().StringVarP(&reportProject, "project", "p", "", "Filter by project")
}

func printWeekReport(wd report.WeekData) {
	_, wn := wd.WeekStart.ISOWeek()
	fmt.Printf("Weekly Report  ·  Week %02d  (%s – %s)\n\n",
		wn,
		wd.WeekStart.Format("Mon Jan 2, 2006"),
		wd.WeekEnd.Format("Mon Jan 2, 2006"),
	)

	if len(wd.Projects) == 0 {
		fmt.Println("No entries this week.")
		return
	}

	printProjectTable(wd.Projects, wd.TotalHours, wd.TotalRevenue)
}

func printMonthReport(md report.MonthData) {
	fmt.Printf("Monthly Report  ·  %s\n\n", md.Month.Format("January 2006"))

	if len(md.Projects) == 0 {
		fmt.Println("No entries this month.")
		return
	}

	// Weekly breakdown
	if len(md.Weeks) > 0 {
		fmt.Println("By week:")
		fmt.Println(strings.Repeat("─", 50))
		for _, w := range md.Weeks {
			_, wn := w.WeekStart.ISOWeek()
			revStr := ""
			if w.TotalRevenue > 0 {
				cur := "USD"
				if len(w.Projects) > 0 {
					cur = w.Projects[0].Currency
				}
				revStr = fmt.Sprintf("   %s", fmtRevenue(w.TotalRevenue, cur))
			}
			fmt.Printf("  Week %02d  (%s – %s)   %5.2fh%s\n",
				wn,
				w.WeekStart.Format("Jan 2"),
				w.WeekEnd.Format("Jan 2"),
				w.TotalHours,
				revStr,
			)
		}
		fmt.Println(strings.Repeat("─", 50))
		fmt.Println()
	}

	fmt.Println("By project:")
	printProjectTable(md.Projects, md.TotalHours, md.TotalRevenue)
}

func printProjectTable(projects []report.ProjectSummary, totalHours, totalRevenue float64) {
	nameW := 10
	for _, p := range projects {
		if len(p.Name) > nameW {
			nameW = len(p.Name)
		}
	}

	showRevenue := totalRevenue > 0
	if showRevenue {
		fmt.Printf("%-*s   Hours     Revenue\n", nameW, "Project")
		fmt.Println(strings.Repeat("─", nameW+24))
		for _, p := range projects {
			fmt.Printf("%-*s   %5.2fh   %s\n", nameW, p.Name, p.Hours, fmtRevenue(p.Revenue, p.Currency))
		}
		fmt.Println(strings.Repeat("─", nameW+24))
		cur := "USD"
		if len(projects) > 0 {
			cur = projects[0].Currency
		}
		fmt.Printf("%-*s   %5.2fh   %s\n", nameW, "Total", totalHours, fmtRevenue(totalRevenue, cur))
	} else {
		fmt.Printf("%-*s   Hours\n", nameW, "Project")
		fmt.Println(strings.Repeat("─", nameW+12))
		for _, p := range projects {
			fmt.Printf("%-*s   %5.2fh\n", nameW, p.Name, p.Hours)
		}
		fmt.Println(strings.Repeat("─", nameW+12))
		fmt.Printf("%-*s   %5.2fh\n", nameW, "Total", totalHours)
	}
}

func fmtRevenue(amount float64, currency string) string {
	switch strings.ToUpper(currency) {
	case "EUR":
		return fmt.Sprintf("€%.2f", amount)
	case "GBP":
		return fmt.Sprintf("£%.2f", amount)
	default:
		return fmt.Sprintf("$%.2f", amount)
	}
}
