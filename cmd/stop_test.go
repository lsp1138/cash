package cmd

import (
	"testing"

	"github.com/larspittman/cash/internal/db"
	"github.com/larspittman/cash/internal/models"
)

func TestStopCreatesBillableTimeEntry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	stopMsg = ""

	d, err := db.Open()
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	if _, err := d.StartTimer("client-project", "implementation"); err != nil {
		d.Close()
		t.Fatalf("StartTimer: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := stopCmd.RunE(stopCmd, nil); err != nil {
		t.Fatalf("stop command: %v", err)
	}

	d, err = db.Open()
	if err != nil {
		t.Fatalf("db.Open after stop: %v", err)
	}
	defer d.Close()
	entries, err := d.GetTimeEntries(models.TimeEntryFilter{ProjectName: "client-project"})
	if err != nil {
		t.Fatalf("GetTimeEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 time entry, got %d", len(entries))
	}
	if !entries[0].Billable {
		t.Fatal("expected timer-created time entry to be billable")
	}
}
