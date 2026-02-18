package report

import (
	"testing"
	"time"

	"github.com/larspittman/cash/internal/models"
)

func monday(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 9, 0, 0, 0, time.UTC)
}

func entry(project string, hours float64, t time.Time) models.TimeEntry {
	return models.TimeEntry{ProjectName: project, Hours: hours, CommittedAt: t}
}

func TestWeekStart(t *testing.T) {
	cases := []struct {
		in   time.Time
		want time.Time
	}{
		{time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC), time.Date(2026, 2, 16, 0, 0, 0, 0, time.UTC)}, // Wed
		{time.Date(2026, 2, 16, 0, 0, 0, 0, time.UTC), time.Date(2026, 2, 16, 0, 0, 0, 0, time.UTC)}, // Mon
		{time.Date(2026, 2, 22, 23, 0, 0, 0, time.UTC), time.Date(2026, 2, 16, 0, 0, 0, 0, time.UTC)}, // Sun
	}
	for _, c := range cases {
		got := WeekStart(c.in)
		if !got.Equal(c.want) {
			t.Errorf("WeekStart(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestBuildWeek_totals(t *testing.T) {
	ws := time.Date(2026, 2, 16, 0, 0, 0, 0, time.UTC) // Monday
	entries := []models.TimeEntry{
		entry("web_dev", 3.0, monday(2026, 2, 16)),  // Mon
		entry("web_dev", 2.5, monday(2026, 2, 17)),  // Tue
		entry("api", 4.0, monday(2026, 2, 18)),       // Wed
		entry("api", 1.5, monday(2026, 2, 20)),       // Fri
	}

	wd := BuildWeek(entries, nil, ws)

	if wd.TotalHours != 11.0 {
		t.Errorf("TotalHours: got %.1f, want 11.0", wd.TotalHours)
	}
	if len(wd.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(wd.Projects))
	}

	// web_dev: 3+2.5=5.5, api: 4+1.5=5.5
	for _, p := range wd.Projects {
		if p.Hours != 5.5 {
			t.Errorf("project %s: hours=%.1f, want 5.5", p.Name, p.Hours)
		}
	}
}

func TestBuildWeek_dailyDistribution(t *testing.T) {
	ws := time.Date(2026, 2, 16, 0, 0, 0, 0, time.UTC)
	entries := []models.TimeEntry{
		entry("p", 2.0, monday(2026, 2, 16)), // Mon idx=0
		entry("p", 3.0, monday(2026, 2, 22)), // Sun idx=6
	}

	wd := BuildWeek(entries, nil, ws)

	if wd.Days[0].Total != 2.0 {
		t.Errorf("Monday total: %.1f, want 2.0", wd.Days[0].Total)
	}
	if wd.Days[6].Total != 3.0 {
		t.Errorf("Sunday total: %.1f, want 3.0", wd.Days[6].Total)
	}
	if wd.Days[1].Total != 0.0 {
		t.Errorf("Tuesday should be empty")
	}
}

func TestBuildWeek_withRates(t *testing.T) {
	ws := time.Date(2026, 2, 16, 0, 0, 0, 0, time.UTC)
	rate := 100.0
	cur := "USD"
	proj := &models.Project{
		Name:       "billable",
		HourlyRate: &rate,
		Customer:   &models.Customer{Currency: cur},
	}
	pm := map[string]*models.Project{"billable": proj}

	entries := []models.TimeEntry{
		entry("billable", 5.0, monday(2026, 2, 16)),
	}
	wd := BuildWeek(entries, pm, ws)

	if wd.TotalRevenue != 500.0 {
		t.Errorf("TotalRevenue: %.2f, want 500.00", wd.TotalRevenue)
	}
}

func TestBuildMonth(t *testing.T) {
	month := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	entries := []models.TimeEntry{
		entry("p", 8.0, monday(2026, 2, 2)),  // week 6
		entry("p", 8.0, monday(2026, 2, 9)),  // week 7
		entry("p", 8.0, monday(2026, 2, 16)), // week 8
		entry("p", 8.0, monday(2026, 2, 23)), // week 9
	}

	md := BuildMonth(entries, nil, month)

	if md.TotalHours != 32.0 {
		t.Errorf("TotalHours: %.1f, want 32.0", md.TotalHours)
	}
	if len(md.Weeks) != 4 {
		t.Errorf("expected 4 weeks, got %d", len(md.Weeks))
	}
}

func TestMonthStartEnd(t *testing.T) {
	d := time.Date(2026, 2, 15, 12, 30, 0, 0, time.UTC)
	ms := MonthStart(d)
	if ms.Day() != 1 || ms.Month() != 2 {
		t.Errorf("MonthStart: %v", ms)
	}
	me := MonthEnd(d)
	if me.Month() != 3 || me.Day() != 1 {
		t.Errorf("MonthEnd: %v", me)
	}
}
