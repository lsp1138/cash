// Package report provides business logic for aggregating time entries.
package report

import (
	"time"

	"github.com/larspittman/cash/internal/models"
)

// ProjectSummary aggregates hours and revenue for one project.
type ProjectSummary struct {
	Name     string
	Hours    float64
	Revenue  float64
	Rate     float64
	Currency string
}

// DayData holds per-project hours for a single calendar day.
type DayData struct {
	Date     time.Time
	Projects map[string]float64 // projectName -> hours
	Total    float64
}

// WeekData holds a full week of aggregated time data.
type WeekData struct {
	WeekStart  time.Time
	WeekEnd    time.Time
	WeekNumber int
	Year       int
	Days       [7]DayData      // Mon[0] … Sun[6]
	Projects   []ProjectSummary
	TotalHours float64
	TotalRevenue float64
}

// MonthData holds a full month of aggregated time data.
type MonthData struct {
	Month        time.Time
	Weeks        []WeekData
	Projects     []ProjectSummary
	TotalHours   float64
	TotalRevenue float64
}

// WeekStart returns the Monday of the ISO week that contains t.
func WeekStart(t time.Time) time.Time {
	t = t.Truncate(24 * time.Hour)
	wd := t.Weekday()
	if wd == time.Sunday {
		wd = 7
	}
	return t.AddDate(0, 0, -int(wd-time.Monday))
}

// MonthStart returns midnight on the first day of t's month.
func MonthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// MonthEnd returns midnight on the first day of the following month.
func MonthEnd(t time.Time) time.Time {
	return MonthStart(t).AddDate(0, 1, 0)
}

// DayIndex maps Go's weekday to Mon=0 … Sun=6.
func DayIndex(wd time.Weekday) int {
	if wd == time.Sunday {
		return 6
	}
	return int(wd) - 1
}

// BuildWeek aggregates entries into a WeekData starting at weekStart (Monday).
func BuildWeek(entries []models.TimeEntry, projectMap map[string]*models.Project, weekStart time.Time) WeekData {
	weekStart = weekStart.Truncate(24 * time.Hour)
	weekEnd := weekStart.AddDate(0, 0, 6)

	wd := WeekData{
		WeekStart: weekStart,
		WeekEnd:   weekEnd,
	}
	wd.Year, wd.WeekNumber = weekStart.ISOWeek()

	// Initialise day data
	for i := range wd.Days {
		wd.Days[i] = DayData{
			Date:     weekStart.AddDate(0, 0, i),
			Projects: make(map[string]float64),
		}
	}

	projectHours := make(map[string]float64)
	projectOrder := []string{}
	seen := make(map[string]bool)

	for _, e := range entries {
		idx := DayIndex(e.CommittedAt.Weekday())
		wd.Days[idx].Projects[e.ProjectName] += e.Hours
		wd.Days[idx].Total += e.Hours
		projectHours[e.ProjectName] += e.Hours
		wd.TotalHours += e.Hours

		if !seen[e.ProjectName] {
			seen[e.ProjectName] = true
			projectOrder = append(projectOrder, e.ProjectName)
		}
	}

	for _, name := range projectOrder {
		hours := projectHours[name]
		rate, currency := projectRate(name, projectMap)
		revenue := hours * rate
		wd.Projects = append(wd.Projects, ProjectSummary{
			Name: name, Hours: hours, Revenue: revenue, Rate: rate, Currency: currency,
		})
		wd.TotalRevenue += revenue
	}

	return wd
}

// BuildMonth aggregates entries into a MonthData for t's calendar month.
func BuildMonth(entries []models.TimeEntry, projectMap map[string]*models.Project, month time.Time) MonthData {
	mstart := MonthStart(month)
	mend := MonthEnd(month)

	md := MonthData{Month: mstart}

	// Iterate over each ISO week that overlaps the month
	wstart := WeekStart(mstart)
	for wstart.Before(mend) {
		wend := wstart.AddDate(0, 0, 7)
		var weekEntries []models.TimeEntry
		for _, e := range entries {
			if !e.CommittedAt.Before(wstart) && e.CommittedAt.Before(wend) {
				weekEntries = append(weekEntries, e)
			}
		}
		if len(weekEntries) > 0 {
			md.Weeks = append(md.Weeks, BuildWeek(weekEntries, projectMap, wstart))
		}
		wstart = wend
	}

	// Overall project totals
	projectHours := make(map[string]float64)
	seen := make(map[string]bool)
	var order []string
	for _, e := range entries {
		projectHours[e.ProjectName] += e.Hours
		md.TotalHours += e.Hours
		if !seen[e.ProjectName] {
			seen[e.ProjectName] = true
			order = append(order, e.ProjectName)
		}
	}
	for _, name := range order {
		hours := projectHours[name]
		rate, currency := projectRate(name, projectMap)
		revenue := hours * rate
		md.Projects = append(md.Projects, ProjectSummary{
			Name: name, Hours: hours, Revenue: revenue, Rate: rate, Currency: currency,
		})
		md.TotalRevenue += revenue
	}

	return md
}

func projectRate(name string, pm map[string]*models.Project) (float64, string) {
	if p, ok := pm[name]; ok && p != nil {
		r, c := p.EffectiveRate()
		return r, c
	}
	return 0, "USD"
}
