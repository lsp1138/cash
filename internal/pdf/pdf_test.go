package pdf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larspittman/cash/internal/config"
	"github.com/larspittman/cash/internal/models"
)

func TestBuildTemplateDataGroupsByProject(t *testing.T) {
	inv := &models.Invoice{
		InvoiceNumber: "INV-2026-001",
		PeriodStart:   time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:     time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
		TotalHours:    5,
		TotalAmount:   343.75,
		Currency:      "EUR",
		CreatedAt:     time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Customer: &models.Customer{
			Name:    "Example Client Ltd",
			Email:   "billing@example.invalid",
			Address: "Example City, Portugal",
		},
		Items: []models.InvoiceItem{
			{ProjectName: "client-app", EntryDate: time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC), Description: "API work", Subservice: "backend", Hours: 2, Rate: 75, Amount: 150, Currency: "EUR"},
			{ProjectName: "client-app", EntryDate: time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC), Description: "UI work", Subservice: "frontend", Hours: 1, Rate: 75, Amount: 75, Currency: "EUR"},
			{ProjectName: "consulting", EntryDate: time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC), Description: "Planning", Subservice: "meeting", Hours: 2, Rate: 75, Amount: 150, Currency: "EUR"},
		},
	}

	data := buildTemplateData(inv, config.Config{Name: "Example Freelancer", Email: "freelancer@example.invalid"})
	if len(data.Groups) != 2 {
		t.Fatalf("expected 2 project groups, got %d", len(data.Groups))
	}
	if data.Groups[0].ProjectName != "client-app" {
		t.Fatalf("expected first group client-app, got %s", data.Groups[0].ProjectName)
	}
	if data.Groups[0].Hours != 3 {
		t.Fatalf("expected client-app subtotal 3h, got %.2f", data.Groups[0].Hours)
	}
	if data.Groups[1].ProjectName != "consulting" {
		t.Fatalf("expected second group consulting, got %s", data.Groups[1].ProjectName)
	}
}

func TestInvoiceTemplateRendersProjectHeadings(t *testing.T) {
	data := invoiceTemplateData{
		InvoiceNumber: "INV-2026-001",
		InvoiceDate:   "May 1, 2026",
		Period:        "Apr 1, 2026 – Apr 30, 2026",
		Status:        "Draft",
		FromName:      "Example Freelancer",
		ToName:        "Example Client Ltd",
		TotalHours:    3,
		TotalAmount:   206.25,
		Currency:      "EUR",
		Groups: []projectGroup{{
			ProjectName: "example-project",
			Hours:       3,
			Amount:      206.25,
			Currency:    "EUR",
			Items: []projectItem{{
				Date: "Apr 03", Description: "API work", Service: "backend", Hours: "3.00", Rate: "€68.75", Amount: "€206.25",
			}},
		}},
	}

	var b strings.Builder
	if err := invoiceTemplate.Execute(&b, data); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := b.String()
	if !strings.Contains(out, "example-project") {
		t.Fatal("expected project heading in rendered html")
	}
	if !strings.Contains(out, "Project subtotal") {
		t.Fatal("expected compact project subtotal row in rendered html")
	}
}

func TestGenerateInvoiceRemovesPartialArtifactsOnBrowserFailure(t *testing.T) {
	fakeBrowser := filepath.Join(t.TempDir(), "fake-browser")
	script := `#!/bin/sh
for arg in "$@"; do
  case "$arg" in
    --print-to-pdf=*) out="${arg#*=}" ;;
  esac
done
printf partial > "$out"
exit 1
`
	if err := os.WriteFile(fakeBrowser, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake browser: %v", err)
	}
	t.Setenv("CASH_PDF_BROWSER", fakeBrowser)

	outputDir := t.TempDir()
	inv := &models.Invoice{
		InvoiceNumber: "INV-FAIL-001",
		PeriodStart:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:     time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
		Currency:      "EUR",
		Status:        models.InvoiceStatusDraft,
		Customer:      &models.Customer{Name: "Example Client"},
	}
	if _, err := GenerateInvoice(inv, config.Config{Name: "Example Freelancer"}, outputDir); err == nil {
		t.Fatal("expected browser failure")
	}
	for _, name := range []string{"INV-FAIL-001.pdf", "INV-FAIL-001.html"} {
		if _, err := os.Stat(filepath.Join(outputDir, name)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, stat error: %v", name, err)
		}
	}
}

func TestGenerateInvoiceCreatesPDFWithHeadlessBrowser(t *testing.T) {
	browser := "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"
	if _, err := os.Stat(browser); err != nil {
		t.Skip("Brave Browser is not installed")
	}
	t.Setenv("CASH_PDF_BROWSER", browser)

	inv := &models.Invoice{
		InvoiceNumber: "INV-TEST-001",
		PeriodStart:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:     time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
		TotalAmount:   1000,
		Currency:      "EUR",
		Status:        models.InvoiceStatusDraft,
		CreatedAt:     time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Customer:      &models.Customer{Name: "Example Client Ltd"},
		Items: []models.InvoiceItem{{
			EntryDate:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			ProjectName: "Example Project",
			Description: "Monthly service",
			Amount:      1000,
			Currency:    "EUR",
		}},
	}
	outputDir := t.TempDir()
	pdfPath, err := GenerateInvoice(inv, config.Config{Name: "Example Freelancer"}, outputDir)
	if err != nil {
		t.Fatalf("GenerateInvoice: %v", err)
	}
	if pdfPath != filepath.Join(outputDir, "INV-TEST-001.pdf") {
		t.Fatalf("unexpected PDF path %q", pdfPath)
	}
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) < 4 || string(data[:4]) != "%PDF" {
		t.Fatalf("expected PDF header, got %q", data[:min(4, len(data))])
	}
}
