package db

import (
	"testing"
	"time"

	"github.com/larspittman/cash/internal/models"
)

func openTest(t *testing.T) *DB {
	t.Helper()
	d, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestAddAndGetTimeEntry(t *testing.T) {
	d := openTest(t)

	e := models.TimeEntry{
		ProjectName: "proj_a",
		Hours:       2.5,
		Message:     "initial work",
		Subservice:  "dev",
		CommittedAt: time.Now(),
	}
	id, err := d.AddTimeEntry(e)
	if err != nil {
		t.Fatalf("AddTimeEntry: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero id")
	}

	entries, err := d.GetTimeEntries(models.TimeEntryFilter{ProjectName: "proj_a"})
	if err != nil {
		t.Fatalf("GetTimeEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Hours != 2.5 {
		t.Errorf("hours: got %.2f, want 2.50", entries[0].Hours)
	}
	if entries[0].Message != "initial work" {
		t.Errorf("message: got %q, want %q", entries[0].Message, "initial work")
	}
}

func TestTimeEntryStartEndTimes(t *testing.T) {
	d := openTest(t)

	start := time.Now().Add(-2 * time.Hour)
	end := time.Now()
	e := models.TimeEntry{
		ProjectName: "proj_b",
		Hours:       2.0,
		StartTime:   &start,
		EndTime:     &end,
		CommittedAt: end,
	}
	if _, err := d.AddTimeEntry(e); err != nil {
		t.Fatalf("AddTimeEntry: %v", err)
	}

	entries, err := d.GetTimeEntries(models.TimeEntryFilter{ProjectName: "proj_b"})
	if err != nil {
		t.Fatalf("GetTimeEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].StartTime == nil {
		t.Error("StartTime should not be nil")
	}
	if entries[0].EndTime == nil {
		t.Error("EndTime should not be nil")
	}
}

func TestTimeEntryDateRangeFilter(t *testing.T) {
	d := openTest(t)

	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)

	d.AddTimeEntry(models.TimeEntry{ProjectName: "p", Hours: 1, CommittedAt: yesterday})
	d.AddTimeEntry(models.TimeEntry{ProjectName: "p", Hours: 2, CommittedAt: now})
	d.AddTimeEntry(models.TimeEntry{ProjectName: "p", Hours: 3, CommittedAt: tomorrow})

	from := now.Truncate(24 * time.Hour)
	to := now.Truncate(24 * time.Hour).Add(48 * time.Hour)
	entries, err := d.GetTimeEntries(models.TimeEntryFilter{From: &from, To: &to})
	if err != nil {
		t.Fatalf("GetTimeEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestCustomerCRUD(t *testing.T) {
	d := openTest(t)

	c := models.Customer{Name: "Acme Corp", Email: "acme@example.com", HourlyRate: 150, Currency: "USD"}
	id, err := d.AddCustomer(c)
	if err != nil {
		t.Fatalf("AddCustomer: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero id")
	}

	all, err := d.GetCustomers()
	if err != nil {
		t.Fatalf("GetCustomers: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1, got %d", len(all))
	}

	found, err := d.GetCustomerByName("acme corp") // case-insensitive
	if err != nil {
		t.Fatalf("GetCustomerByName: %v", err)
	}
	if found == nil {
		t.Fatal("customer not found")
	}
	if found.HourlyRate != 150 {
		t.Errorf("rate: got %.0f, want 150", found.HourlyRate)
	}

	found.ID = id
	found.Address = "123 Business Ave"
	if err := d.UpdateCustomer(*found); err != nil {
		t.Fatalf("UpdateCustomer: %v", err)
	}
	updated, _ := d.GetCustomerByName("Acme Corp")
	if updated.Address != "123 Business Ave" {
		t.Errorf("address: got %q, want %q", updated.Address, "123 Business Ave")
	}
}

func TestCustomerNotFound(t *testing.T) {
	d := openTest(t)
	c, err := d.GetCustomerByName("nonexistent")
	if err != nil {
		t.Fatalf("GetCustomerByName: %v", err)
	}
	if c != nil {
		t.Error("expected nil for nonexistent customer")
	}
}

func TestCustomerSlugGenerationAndLookup(t *testing.T) {
	d := openTest(t)

	id, err := d.AddCustomer(models.Customer{Name: "Acme & Sons, Inc."})
	if err != nil {
		t.Fatalf("AddCustomer: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	byName, err := d.GetCustomerByName("Acme & Sons, Inc.")
	if err != nil {
		t.Fatalf("GetCustomerByName: %v", err)
	}
	if byName == nil {
		t.Fatal("expected customer by name")
	}
	if byName.Slug != "acme_sons_inc" {
		t.Fatalf("slug: got %q, want %q", byName.Slug, "acme_sons_inc")
	}

	byLookupSlug, err := d.GetCustomerBySlugOrName("acme_sons_inc")
	if err != nil {
		t.Fatalf("GetCustomerBySlugOrName(slug): %v", err)
	}
	if byLookupSlug == nil || byLookupSlug.ID != byName.ID {
		t.Fatal("expected slug lookup to match customer")
	}

	byLookupName, err := d.GetCustomerBySlugOrName("Acme & Sons, Inc.")
	if err != nil {
		t.Fatalf("GetCustomerBySlugOrName(name): %v", err)
	}
	if byLookupName == nil || byLookupName.ID != byName.ID {
		t.Fatal("expected name lookup to match customer")
	}
}

func TestCustomerSlugManualValueNormalized(t *testing.T) {
	d := openTest(t)

	_, err := d.AddCustomer(models.Customer{Name: "Beta Co", Slug: "  Beta! Co + 2026  "})
	if err != nil {
		t.Fatalf("AddCustomer: %v", err)
	}
	c, err := d.GetCustomerByName("Beta Co")
	if err != nil {
		t.Fatalf("GetCustomerByName: %v", err)
	}
	if c == nil {
		t.Fatal("expected customer")
	}
	if c.Slug != "beta_co_2026" {
		t.Fatalf("slug: got %q, want %q", c.Slug, "beta_co_2026")
	}
}

func TestProjectCRUD(t *testing.T) {
	d := openTest(t)

	p := models.Project{Name: "web_app", Description: "Main website"}
	id, err := d.AddProject(p)
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero id")
	}

	all, err := d.GetProjects()
	if err != nil {
		t.Fatalf("GetProjects: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1, got %d", len(all))
	}

	found, err := d.GetProjectByName("web_app")
	if err != nil {
		t.Fatalf("GetProjectByName: %v", err)
	}
	if found == nil {
		t.Fatal("project not found")
	}
	if found.Description != "Main website" {
		t.Errorf("desc: got %q, want %q", found.Description, "Main website")
	}
}

func TestProjectWithCustomer(t *testing.T) {
	d := openTest(t)

	cid, _ := d.AddCustomer(models.Customer{Name: "BigCo", HourlyRate: 200, Currency: "EUR"})
	rate := 250.0
	p := models.Project{Name: "bigco_api", CustomerID: &cid, HourlyRate: &rate}
	d.AddProject(p)

	found, err := d.GetProjectByName("bigco_api")
	if err != nil {
		t.Fatalf("GetProjectByName: %v", err)
	}
	if found.Customer == nil {
		t.Fatal("expected customer to be loaded")
	}
	if found.Customer.Name != "BigCo" {
		t.Errorf("customer name: got %q, want BigCo", found.Customer.Name)
	}
	if found.HourlyRate == nil || *found.HourlyRate != 250 {
		t.Error("expected project hourly rate of 250")
	}
}

func TestTimerOperations(t *testing.T) {
	d := openTest(t)

	// No timer initially
	timer, err := d.GetActiveTimer()
	if err != nil {
		t.Fatalf("GetActiveTimer: %v", err)
	}
	if timer != nil {
		t.Error("expected nil timer initially")
	}

	id, err := d.StartTimer("proj_x", "working on feature")
	if err != nil {
		t.Fatalf("StartTimer: %v", err)
	}

	timer, err = d.GetActiveTimer()
	if err != nil {
		t.Fatalf("GetActiveTimer: %v", err)
	}
	if timer == nil {
		t.Fatal("expected active timer")
	}
	if timer.ProjectName != "proj_x" {
		t.Errorf("project: got %q, want proj_x", timer.ProjectName)
	}

	if err := d.DeleteTimer(id); err != nil {
		t.Fatalf("DeleteTimer: %v", err)
	}
	timer, _ = d.GetActiveTimer()
	if timer != nil {
		t.Error("expected nil timer after deletion")
	}
}

func TestNextInvoiceNumber(t *testing.T) {
	d := openTest(t)

	num, err := d.NextInvoiceNumber(2026)
	if err != nil {
		t.Fatalf("NextInvoiceNumber: %v", err)
	}
	if num != "INV-2026-001" {
		t.Errorf("got %q, want INV-2026-001", num)
	}

	cid, _ := d.AddCustomer(models.Customer{Name: "TestCo", Currency: "USD"})
	d.AddInvoice(models.Invoice{
		CustomerID:    cid,
		InvoiceNumber: "INV-2026-001",
		PeriodStart:   time.Now(),
		PeriodEnd:     time.Now(),
		Currency:      "USD",
		Status:        "draft",
	})

	num2, err := d.NextInvoiceNumber(2026)
	if err != nil {
		t.Fatalf("NextInvoiceNumber: %v", err)
	}
	if num2 != "INV-2026-002" {
		t.Errorf("got %q, want INV-2026-002", num2)
	}
}
