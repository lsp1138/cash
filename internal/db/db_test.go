package db

import (
	"os"
	"path/filepath"
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

func TestDataDirAndDatabaseUsePrivatePermissions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	d, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	dirInfo, err := os.Stat(filepath.Join(home, ".cash"))
	if err != nil {
		t.Fatalf("Stat .cash: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf(".cash mode: got %04o, want 0700", got)
	}
	dbInfo, err := os.Stat(filepath.Join(home, ".cash", "cash.db"))
	if err != nil {
		t.Fatalf("Stat cash.db: %v", err)
	}
	if got := dbInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("cash.db mode: got %04o, want 0600", got)
	}
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

func TestAssignTimeEntriesToInvoiceAndGetByInvoiceID(t *testing.T) {
	d := openTest(t)

	firstID, err := d.AddTimeEntry(models.TimeEntry{
		ProjectName: "proj_a",
		Hours:       2,
		Message:     "feature work",
		Billable:    true,
		CommittedAt: time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("AddTimeEntry(first): %v", err)
	}
	secondID, err := d.AddTimeEntry(models.TimeEntry{
		ProjectName: "proj_a",
		Hours:       1.5,
		Message:     "bugfix",
		Billable:    true,
		CommittedAt: time.Date(2026, 1, 11, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("AddTimeEntry(second): %v", err)
	}

	customerID, err := d.AddCustomer(models.Customer{Name: "Acme Corp", Currency: "EUR"})
	if err != nil {
		t.Fatalf("AddCustomer: %v", err)
	}
	invoiceID, err := d.AddInvoice(models.Invoice{
		CustomerID:    customerID,
		InvoiceNumber: "INV-2026-001",
		PeriodStart:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:     time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
		Currency:      "EUR",
		Status:        "draft",
	})
	if err != nil {
		t.Fatalf("AddInvoice: %v", err)
	}

	if err := d.AssignTimeEntriesToInvoice(invoiceID, []int64{firstID, secondID}); err != nil {
		t.Fatalf("AssignTimeEntriesToInvoice: %v", err)
	}

	entries, err := d.GetTimeEntriesByInvoiceID(invoiceID)
	if err != nil {
		t.Fatalf("GetTimeEntriesByInvoiceID: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].InvoiceID == nil || *entries[0].InvoiceID != invoiceID {
		t.Fatal("expected first entry to be linked to invoice")
	}
	if entries[1].InvoiceID == nil || *entries[1].InvoiceID != invoiceID {
		t.Fatal("expected second entry to be linked to invoice")
	}
}

func TestInvoiceItemsStoredAndLoadedWithInvoice(t *testing.T) {
	d := openTest(t)

	customerID, err := d.AddCustomer(models.Customer{Name: "Acme Corp", Currency: "EUR", HourlyRate: 100})
	if err != nil {
		t.Fatalf("AddCustomer: %v", err)
	}
	invoiceID, err := d.AddInvoice(models.Invoice{
		CustomerID:    customerID,
		InvoiceNumber: "INV-2026-010",
		PeriodStart:   time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:     time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC),
		TotalHours:    3.5,
		TotalAmount:   350,
		Currency:      "EUR",
		Status:        models.InvoiceStatusDraft,
	})
	if err != nil {
		t.Fatalf("AddInvoice: %v", err)
	}

	entryID := int64(42)
	if err := d.AddInvoiceItems(invoiceID, []models.InvoiceItem{{
		TimeEntryID: &entryID,
		EntryDate:   time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC),
		ProjectName: "proj_a",
		Description: "API work",
		Subservice:  "backend",
		Hours:       3.5,
		Rate:        100,
		Amount:      350,
		Currency:    "EUR",
	}}); err != nil {
		t.Fatalf("AddInvoiceItems: %v", err)
	}

	inv, err := d.GetInvoiceByNumber("INV-2026-010")
	if err != nil {
		t.Fatalf("GetInvoiceByNumber: %v", err)
	}
	if inv == nil {
		t.Fatal("expected invoice")
	}
	if len(inv.Items) != 1 {
		t.Fatalf("expected 1 invoice item, got %d", len(inv.Items))
	}
	if inv.Items[0].Amount != 350 {
		t.Fatalf("expected amount 350, got %.2f", inv.Items[0].Amount)
	}
}

func TestInvoiceTaxReferenceStoredAndUpdated(t *testing.T) {
	d := openTest(t)

	customerID, err := d.AddCustomer(models.Customer{Name: "Acme Corp", Currency: "EUR"})
	if err != nil {
		t.Fatalf("AddCustomer: %v", err)
	}
	invoiceID, err := d.AddInvoice(models.Invoice{
		CustomerID:    customerID,
		InvoiceNumber: "INV-2026-020",
		TaxReference:  "FT A/1",
		PeriodStart:   time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:     time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
		Currency:      "EUR",
		Status:        models.InvoiceStatusDraft,
	})
	if err != nil {
		t.Fatalf("AddInvoice: %v", err)
	}
	if err := d.UpdateInvoiceTaxReference(invoiceID, "FT A/2"); err != nil {
		t.Fatalf("UpdateInvoiceTaxReference: %v", err)
	}

	inv, err := d.GetInvoiceByNumber("INV-2026-020")
	if err != nil {
		t.Fatalf("GetInvoiceByNumber: %v", err)
	}
	if inv == nil {
		t.Fatal("expected invoice")
	}
	if inv.TaxReference != "FT A/2" {
		t.Fatalf("expected tax ref FT A/2, got %q", inv.TaxReference)
	}
}

func TestInvoiceCancelLifecycleReleasesEntries(t *testing.T) {
	d := openTest(t)

	entryID, err := d.AddTimeEntry(models.TimeEntry{
		ProjectName: "proj_a",
		Hours:       2,
		Message:     "feature work",
		Billable:    true,
		CommittedAt: time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("AddTimeEntry: %v", err)
	}

	customerID, err := d.AddCustomer(models.Customer{Name: "Beta Co", Currency: "EUR"})
	if err != nil {
		t.Fatalf("AddCustomer: %v", err)
	}
	invoiceID, err := d.AddInvoice(models.Invoice{
		CustomerID:    customerID,
		InvoiceNumber: "INV-2026-011",
		PeriodStart:   time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:     time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		Currency:      "EUR",
		Status:        models.InvoiceStatusDraft,
	})
	if err != nil {
		t.Fatalf("AddInvoice: %v", err)
	}
	if err := d.AssignTimeEntriesToInvoice(invoiceID, []int64{entryID}); err != nil {
		t.Fatalf("AssignTimeEntriesToInvoice: %v", err)
	}

	cancelledAt := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	if err := d.CancelInvoice(invoiceID, cancelledAt); err != nil {
		t.Fatalf("CancelInvoice: %v", err)
	}

	entries, err := d.GetTimeEntries(models.TimeEntryFilter{ProjectName: "proj_a"})
	if err != nil {
		t.Fatalf("GetTimeEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].InvoiceID != nil {
		t.Fatal("expected entry to be released from invoice")
	}

	inv, err := d.GetInvoiceByNumber("INV-2026-011")
	if err != nil {
		t.Fatalf("GetInvoiceByNumber: %v", err)
	}
	if inv.Status != models.InvoiceStatusCancelled {
		t.Fatalf("expected cancelled status, got %s", inv.Status)
	}
	if inv.CancelledAt == nil || !inv.CancelledAt.Equal(cancelledAt) {
		t.Fatal("expected cancelled timestamp to be set")
	}
}

func TestRecurringChargeEntriesCanBeGeneratedAndInvoiced(t *testing.T) {
	d := openTest(t)

	customerID, err := d.AddCustomer(models.Customer{Name: "Example Client Ltd", Currency: "EUR"})
	if err != nil {
		t.Fatalf("AddCustomer: %v", err)
	}
	if _, err := d.AddProject(models.Project{Name: "Platform Retainer", CustomerID: &customerID}); err != nil {
		t.Fatalf("AddProject: %v", err)
	}

	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	if _, err := d.conn.Exec(`
		INSERT INTO recurring_charge_definitions
		 (customer_id, project_name, subservice, description, amount, currency, billable, cadence, start_month, active)
		 VALUES (?, 'Platform Retainer', 'Platform delivery', 'Monthly platform delivery fee', 1000, 'EUR', 1, 'monthly', '2026-06-01', 1)`,
		customerID,
	); err != nil {
		t.Fatalf("insert recurring definition: %v", err)
	}
	if err := d.EnsureRecurringChargeEntriesForMonth(now); err != nil {
		t.Fatalf("EnsureRecurringChargeEntriesForMonth: %v", err)
	}
	if err := d.EnsureRecurringChargeEntriesForMonth(now); err != nil {
		t.Fatalf("EnsureRecurringChargeEntriesForMonth(second): %v", err)
	}

	entries, err := d.GetRecurringChargeEntries(customerID,
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("GetRecurringChargeEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 recurring entry, got %d", len(entries))
	}
	if entries[0].Amount != 1000 {
		t.Fatalf("expected recurring amount 1000, got %.2f", entries[0].Amount)
	}
	if entries[0].ProjectName != "Platform Retainer" {
		t.Fatalf("unexpected project %q", entries[0].ProjectName)
	}

	invoiceID, err := d.AddInvoice(models.Invoice{
		CustomerID:    customerID,
		InvoiceNumber: "INV-2026-099",
		PeriodStart:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:     time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		TotalHours:    0,
		TotalAmount:   1000,
		Currency:      "EUR",
		Status:        models.InvoiceStatusDraft,
	})
	if err != nil {
		t.Fatalf("AddInvoice: %v", err)
	}
	if err := d.AssignRecurringChargeEntriesToInvoice(invoiceID, []int64{entries[0].ID}); err != nil {
		t.Fatalf("AssignRecurringChargeEntriesToInvoice: %v", err)
	}
	if err := d.AddInvoiceItems(invoiceID, []models.InvoiceItem{{
		EntryDate:   entries[0].PeriodStart,
		ProjectName: entries[0].ProjectName,
		Description: entries[0].Description,
		Subservice:  entries[0].Subservice,
		Hours:       0,
		Rate:        0,
		Amount:      entries[0].Amount,
		Currency:    entries[0].Currency,
	}}); err != nil {
		t.Fatalf("AddInvoiceItems: %v", err)
	}

	if err := d.CancelInvoice(invoiceID, time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("CancelInvoice: %v", err)
	}
	entries, err = d.GetRecurringChargeEntries(customerID,
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("GetRecurringChargeEntries(after cancel): %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 recurring entry after cancel, got %d", len(entries))
	}
	if entries[0].InvoiceID != nil {
		t.Fatal("expected recurring entry to be released from invoice")
	}
}

func TestMustParseDateSupportsTimestampShapes(t *testing.T) {
	cases := []string{
		"2026-04-01",
		"2026-04-01T00:00:00Z",
		"2026-04-01 00:00:00",
		"2026-04-01 00:00:00 +0000 UTC",
	}
	for _, c := range cases {
		got := mustParseDate(c)
		if got.IsZero() {
			t.Fatalf("expected %q to parse", c)
		}
		if got.Format("2006-01-02") != "2026-04-01" {
			t.Fatalf("unexpected parsed date for %q: %s", c, got.Format("2006-01-02"))
		}
	}
}

func TestCreateInvoiceRollsBackOnMissingTimeEntry(t *testing.T) {
	d := openTest(t)
	customerID, err := d.AddCustomer(models.Customer{Name: "Example Client", Currency: "EUR"})
	if err != nil {
		t.Fatalf("AddCustomer: %v", err)
	}
	inv := models.Invoice{
		CustomerID:    customerID,
		InvoiceNumber: "INV-2026-500",
		PeriodStart:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:     time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
		TotalAmount:   100,
		Currency:      "EUR",
		Status:        models.InvoiceStatusDraft,
		PDFPath:       "/tmp/INV-2026-500.pdf",
	}

	if _, err := d.CreateInvoice(inv, []int64{999}, nil, []models.InvoiceItem{{
		EntryDate:   inv.PeriodStart,
		ProjectName: "missing-project",
		Amount:      100,
		Currency:    "EUR",
	}}); err == nil {
		t.Fatal("expected missing time entry error")
	}
	invoices, err := d.GetInvoices()
	if err != nil {
		t.Fatalf("GetInvoices: %v", err)
	}
	if len(invoices) != 0 {
		t.Fatalf("expected transaction rollback, got %d invoices", len(invoices))
	}
}

func TestSettingsCanBeStoredAndLoaded(t *testing.T) {
	d := openTest(t)

	if err := d.SetSetting("email", "freelancer@example.invalid"); err != nil {
		t.Fatalf("SetSetting(email): %v", err)
	}
	if err := d.SetSetting("phone", "+000 000 000 000"); err != nil {
		t.Fatalf("SetSetting(phone): %v", err)
	}

	settings, err := d.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if settings["email"] != "freelancer@example.invalid" {
		t.Fatalf("unexpected email %q", settings["email"])
	}
	if settings["phone"] != "+000 000 000 000" {
		t.Fatalf("unexpected phone %q", settings["phone"])
	}
}
