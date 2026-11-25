package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larspittman/cash/internal/db"
	"github.com/larspittman/cash/internal/models"
)

func resetInvoiceGenerateFlags() {
	invCustomer = ""
	invFrom = ""
	invTo = ""
	invNumber = ""
	invRate = 0
	invMonth = ""
	invWeek = ""
	invAll = false
	invEntries = nil
	invInclude = nil
	invExclude = nil
}

func TestInvoiceScopeMonth(t *testing.T) {
	resetInvoiceGenerateFlags()
	invMonth = "2026-05"

	from, to, label, err := invoiceScope()
	if err != nil {
		t.Fatalf("invoiceScope: %v", err)
	}
	if got, want := from.Format("2006-01-02"), "2026-05-01"; got != want {
		t.Fatalf("from: got %s want %s", got, want)
	}
	if got, want := to.Format("2006-01-02"), "2026-06-01"; got != want {
		t.Fatalf("to: got %s want %s", got, want)
	}
	if label != "May 2026" {
		t.Fatalf("label: got %q", label)
	}
}

func TestInvoiceScopeRejectsMixedScopes(t *testing.T) {
	resetInvoiceGenerateFlags()
	invMonth = "2026-05"
	invFrom = "2026-05-01"
	invTo = "2026-05-31"

	if _, _, _, err := invoiceScope(); err == nil {
		t.Fatal("expected mixed scope error")
	}
}

func TestInvoiceGenerationPDFErrorLeavesDatabaseUnchanged(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CASH_PDF_BROWSER", "/definitely/missing-browser")
	resetInvoiceGenerateFlags()

	d, err := db.Open()
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	customerID, err := d.AddCustomer(models.Customer{Name: "Acme Corp", Currency: "EUR", HourlyRate: 100})
	if err != nil {
		d.Close()
		t.Fatalf("AddCustomer: %v", err)
	}
	if _, err := d.AddProject(models.Project{Name: "client-project", CustomerID: &customerID}); err != nil {
		d.Close()
		t.Fatalf("AddProject: %v", err)
	}
	entryID, err := d.AddTimeEntry(models.TimeEntry{
		ProjectName: "client-project",
		Hours:       2,
		Message:     "Implementation",
		Billable:    true,
		CommittedAt: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		d.Close()
		t.Fatalf("AddTimeEntry: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	invCustomer = "Acme Corp"
	invMonth = "2026-07"
	invAll = true
	err = invoiceGenerateCmd.RunE(invoiceGenerateCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "generating PDF") {
		t.Fatalf("expected PDF generation error, got %v", err)
	}

	d, err = db.Open()
	if err != nil {
		t.Fatalf("db.Open after failure: %v", err)
	}
	defer d.Close()
	invoices, err := d.GetInvoices()
	if err != nil {
		t.Fatalf("GetInvoices: %v", err)
	}
	if len(invoices) != 0 {
		t.Fatalf("expected no invoice after PDF failure, got %d", len(invoices))
	}
	entries, err := d.GetTimeEntries(models.TimeEntryFilter{ProjectName: "client-project"})
	if err != nil {
		t.Fatalf("GetTimeEntries: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != entryID {
		t.Fatalf("expected original entry %d, got %+v", entryID, entries)
	}
	if entries[0].InvoiceID != nil {
		t.Fatalf("expected entry to remain uninvoiced, got invoice %d", *entries[0].InvoiceID)
	}
}

func TestInvoiceGenerationRejectsDuplicateNumberBeforeRendering(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CASH_PDF_BROWSER", "/definitely/missing-browser")
	resetInvoiceGenerateFlags()

	d, err := db.Open()
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	customerID, err := d.AddCustomer(models.Customer{Name: "Example Client", Currency: "EUR", HourlyRate: 100})
	if err != nil {
		d.Close()
		t.Fatalf("AddCustomer: %v", err)
	}
	if _, err := d.AddProject(models.Project{Name: "client-project", CustomerID: &customerID}); err != nil {
		d.Close()
		t.Fatalf("AddProject: %v", err)
	}
	if _, err := d.AddTimeEntry(models.TimeEntry{ProjectName: "client-project", Hours: 1, Billable: true, CommittedAt: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)}); err != nil {
		d.Close()
		t.Fatalf("AddTimeEntry: %v", err)
	}
	if _, err := d.AddInvoice(models.Invoice{CustomerID: customerID, InvoiceNumber: "INV-2026-777", PeriodStart: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), PeriodEnd: time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC), Currency: "EUR", Status: models.InvoiceStatusDraft}); err != nil {
		d.Close()
		t.Fatalf("AddInvoice: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	invCustomer = "Example Client"
	invMonth = "2026-07"
	invAll = true
	invNumber = "INV-2026-777"
	err = invoiceGenerateCmd.RunE(invoiceGenerateCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate-number error before rendering, got %v", err)
	}
}

func TestSelectInvoiceProjectsIncludeExclude(t *testing.T) {
	projects := []models.Project{
		{Name: "client-app"},
		{Name: "consulting"},
		{Name: "vision-2026"},
	}

	selected, err := selectInvoiceProjects(projects, []string{"client-app", "consulting"}, []string{"vision-2026"})
	if err != nil {
		t.Fatalf("selectInvoiceProjects: %v", err)
	}
	if len(selected) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(selected))
	}
	if selected[0].Name != "client-app" || selected[1].Name != "consulting" {
		t.Fatalf("unexpected project selection: %+v", selected)
	}
}

func TestSelectInvoiceEntriesByID(t *testing.T) {
	entries := []models.TimeEntry{
		{ID: 10, ProjectName: "client-app", CommittedAt: time.Now()},
		{ID: 11, ProjectName: "client-app", CommittedAt: time.Now()},
		{ID: 12, ProjectName: "consulting", CommittedAt: time.Now()},
	}

	selected, err := selectInvoiceEntries(entries, []int64{12, 10})
	if err != nil {
		t.Fatalf("selectInvoiceEntries: %v", err)
	}
	if len(selected) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(selected))
	}
	if selected[0].ID != 10 || selected[1].ID != 12 {
		t.Fatalf("unexpected selected entries: %+v", selected)
	}
}

// fakeBrowser writes a shell script that copies the rendered HTML into the
// requested PDF path, standing in for a Chromium binary.
func fakeBrowser(t *testing.T) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "browser.sh")
	body := `#!/bin/sh
out=""
src=""
for arg in "$@"; do
  case "$arg" in
    --print-to-pdf=*) out="${arg#--print-to-pdf=}" ;;
    file://*) src="${arg#file://}" ;;
  esac
done
cp "$src" "$out"
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake browser: %v", err)
	}
	return script
}

func seedDraftInvoice(t *testing.T) string {
	t.Helper()
	d, err := db.Open()
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer d.Close()
	customerID, err := d.AddCustomer(models.Customer{Name: "Acme Corp", Currency: "EUR", HourlyRate: 100})
	if err != nil {
		t.Fatalf("AddCustomer: %v", err)
	}
	if _, err := d.AddInvoice(models.Invoice{
		CustomerID:    customerID,
		InvoiceNumber: "INV-TEST-001",
		PeriodStart:   time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:     time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC),
		TotalAmount:   1000,
		Currency:      "EUR",
		Status:        models.InvoiceStatusDraft,
	}); err != nil {
		t.Fatalf("AddInvoice: %v", err)
	}
	return "INV-TEST-001"
}

func loadInvoice(t *testing.T, number string) *models.Invoice {
	t.Helper()
	d, err := db.Open()
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer d.Close()
	inv, err := d.GetInvoiceByNumber(number)
	if err != nil || inv == nil {
		t.Fatalf("GetInvoiceByNumber: %v %v", inv, err)
	}
	return inv
}

func TestInvoiceSendPDFErrorLeavesStatusUnchanged(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CASH_PDF_BROWSER", "/definitely/missing-browser")
	number := seedDraftInvoice(t)

	err := invoiceSendCmd.RunE(invoiceSendCmd, []string{number})
	if err == nil || !strings.Contains(err.Error(), "generating PDF") {
		t.Fatalf("expected PDF generation error, got %v", err)
	}
	inv := loadInvoice(t, number)
	if inv.Status != models.InvoiceStatusDraft || inv.SentAt != nil || inv.PDFPath != "" {
		t.Fatalf("expected untouched draft, got status=%s sentAt=%v pdf=%q", inv.Status, inv.SentAt, inv.PDFPath)
	}
}

func TestInvoiceTransitionsRegeneratePDFWithNewStatus(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CASH_PDF_BROWSER", fakeBrowser(t))
	number := seedDraftInvoice(t)

	expect := func(step, needle string) {
		t.Helper()
		inv := loadInvoice(t, number)
		if inv.PDFPath == "" {
			t.Fatalf("%s: expected PDF path to be stored", step)
		}
		rendered, err := os.ReadFile(inv.PDFPath)
		if err != nil {
			t.Fatalf("%s: read PDF: %v", step, err)
		}
		if !strings.Contains(string(rendered), needle) {
			t.Fatalf("%s: rendered PDF does not contain %q", step, needle)
		}
	}

	if err := invoiceTaxRefCmd.RunE(invoiceTaxRefCmd, []string{number, "FR TEST/1"}); err != nil {
		t.Fatalf("tax-ref: %v", err)
	}
	expect("tax-ref", "FR TEST/1")
	if loadInvoice(t, number).Status != models.InvoiceStatusDraft {
		t.Fatalf("tax-ref must not change status")
	}

	if err := invoiceSendCmd.RunE(invoiceSendCmd, []string{number}); err != nil {
		t.Fatalf("send: %v", err)
	}
	expect("send", "<strong>Sent</strong>")

	invPaidAt = "2026-12-01"
	t.Cleanup(func() { invPaidAt = "" })
	if err := invoicePaidCmd.RunE(invoicePaidCmd, []string{number}); err != nil {
		t.Fatalf("paid: %v", err)
	}
	expect("paid", "<strong>Paid</strong>")
	inv := loadInvoice(t, number)
	if inv.Status != models.InvoiceStatusPaid || inv.PaidAt == nil || inv.PaidAt.Format("2006-01-02") != "2026-12-01" {
		t.Fatalf("expected paid on 2026-12-01, got status=%s paidAt=%v", inv.Status, inv.PaidAt)
	}
}
