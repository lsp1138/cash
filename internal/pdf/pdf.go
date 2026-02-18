// Package pdf generates invoice PDF documents.
package pdf

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"

	"github.com/larspittman/cash/internal/config"
	"github.com/larspittman/cash/internal/models"
)

const (
	pageW  = 210.0 // A4 width mm
	margin = 20.0
	colW   = pageW - 2*margin
)

// GenerateInvoice creates a PDF for the given invoice and saves it to outputPath.
// It returns the full path of the written file.
func GenerateInvoice(inv *models.Invoice, cfg config.Config, outputDir string) (string, error) {
	filename := fmt.Sprintf("%s.pdf", inv.InvoiceNumber)
	outPath := filepath.Join(outputDir, filename)

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(margin, margin, margin)
	pdf.AddPage()

	setFont := func(style string, size float64) {
		pdf.SetFont("Helvetica", style, size)
	}

	// ── Header ────────────────────────────────────────────────────────────
	setFont("B", 26)
	pdf.SetTextColor(30, 30, 30)
	pdf.CellFormat(colW, 12, "INVOICE", "", 1, "R", false, 0, "")
	pdf.Ln(2)

	// Invoice meta
	setFont("", 10)
	pdf.SetTextColor(80, 80, 80)
	pdf.CellFormat(colW, 6, fmt.Sprintf("Invoice Number: %s", inv.InvoiceNumber), "", 1, "R", false, 0, "")
	pdf.CellFormat(colW, 6, fmt.Sprintf("Date: %s", time.Now().Format("January 2, 2006")), "", 1, "R", false, 0, "")
	pdf.CellFormat(colW, 6, fmt.Sprintf("Period: %s – %s",
		inv.PeriodStart.Format("Jan 2, 2006"),
		inv.PeriodEnd.Format("Jan 2, 2006"),
	), "", 1, "R", false, 0, "")
	pdf.Ln(6)

	// Divider
	pdf.SetDrawColor(200, 200, 200)
	pdf.Line(margin, pdf.GetY(), pageW-margin, pdf.GetY())
	pdf.Ln(6)

	// ── FROM / TO columns ─────────────────────────────────────────────────
	fromX := margin
	toX := margin + colW/2 + 5
	startY := pdf.GetY()

	// FROM
	pdf.SetXY(fromX, startY)
	setFont("B", 9)
	pdf.SetTextColor(120, 120, 120)
	pdf.CellFormat(colW/2-5, 5, "FROM", "", 1, "L", false, 0, "")
	setFont("B", 11)
	pdf.SetTextColor(30, 30, 30)
	pdf.SetX(fromX)
	pdf.CellFormat(colW/2-5, 6, cfg.Name, "", 1, "L", false, 0, "")
	setFont("", 9)
	pdf.SetTextColor(60, 60, 60)
	if cfg.Email != "" {
		pdf.SetX(fromX)
		pdf.CellFormat(colW/2-5, 5, cfg.Email, "", 1, "L", false, 0, "")
	}
	if cfg.Address != "" {
		for _, line := range strings.Split(cfg.Address, "\n") {
			pdf.SetX(fromX)
			pdf.CellFormat(colW/2-5, 5, line, "", 1, "L", false, 0, "")
		}
	}

	// TO
	pdf.SetXY(toX, startY)
	setFont("B", 9)
	pdf.SetTextColor(120, 120, 120)
	pdf.CellFormat(colW/2-5, 5, "TO", "", 1, "L", false, 0, "")
	setFont("B", 11)
	pdf.SetTextColor(30, 30, 30)
	pdf.SetX(toX)
	if inv.Customer != nil {
		pdf.CellFormat(colW/2-5, 6, inv.Customer.Name, "", 1, "L", false, 0, "")
		setFont("", 9)
		pdf.SetTextColor(60, 60, 60)
		if inv.Customer.Email != "" {
			pdf.SetX(toX)
			pdf.CellFormat(colW/2-5, 5, inv.Customer.Email, "", 1, "L", false, 0, "")
		}
		if inv.Customer.Address != "" {
			for _, line := range strings.Split(inv.Customer.Address, "\n") {
				pdf.SetX(toX)
				pdf.CellFormat(colW/2-5, 5, line, "", 1, "L", false, 0, "")
			}
		}
	}

	pdf.Ln(10)

	// ── Time Entries Table ─────────────────────────────────────────────────
	tableY := pdf.GetY()
	pdf.SetY(tableY)

	// Column widths: Date | Project | Description | Sub | Hours | Rate | Amount
	cDate := 25.0
	cProj := 35.0
	cDesc := 55.0
	cSub := 20.0
	cHrs := 15.0
	cRate := 20.0
	cAmt := colW - cDate - cProj - cDesc - cSub - cHrs - cRate

	// Table header
	pdf.SetFillColor(45, 45, 45)
	pdf.SetTextColor(255, 255, 255)
	setFont("B", 8)
	rowH := 7.0
	pdf.CellFormat(cDate, rowH, "Date", "0", 0, "L", true, 0, "")
	pdf.CellFormat(cProj, rowH, "Project", "0", 0, "L", true, 0, "")
	pdf.CellFormat(cDesc, rowH, "Description", "0", 0, "L", true, 0, "")
	pdf.CellFormat(cSub, rowH, "Category", "0", 0, "L", true, 0, "")
	pdf.CellFormat(cHrs, rowH, "Hours", "0", 0, "R", true, 0, "")
	pdf.CellFormat(cRate, rowH, "Rate", "0", 0, "R", true, 0, "")
	pdf.CellFormat(cAmt, rowH, "Amount", "0", 1, "R", true, 0, "")

	// Table rows
	setFont("", 8)
	pdf.SetTextColor(30, 30, 30)
	for i, e := range inv.Entries {
		if i%2 == 0 {
			pdf.SetFillColor(248, 248, 248)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		rate, currency := projectRate(e.ProjectName, inv)
		amount := e.Hours * rate

		pdf.CellFormat(cDate, rowH, e.CommittedAt.Format("Jan 02"), "0", 0, "L", true, 0, "")
		pdf.CellFormat(cProj, rowH, truncate(e.ProjectName, 18), "0", 0, "L", true, 0, "")
		pdf.CellFormat(cDesc, rowH, truncate(e.Message, 28), "0", 0, "L", true, 0, "")
		pdf.CellFormat(cSub, rowH, truncate(e.Subservice, 10), "0", 0, "L", true, 0, "")
		pdf.CellFormat(cHrs, rowH, fmt.Sprintf("%.2f", e.Hours), "0", 0, "R", true, 0, "")
		pdf.CellFormat(cRate, rowH, fmtMoney(rate, currency), "0", 0, "R", true, 0, "")
		pdf.CellFormat(cAmt, rowH, fmtMoney(amount, currency), "0", 1, "R", true, 0, "")
	}

	pdf.Ln(4)

	// ── Totals ────────────────────────────────────────────────────────────
	totalX := margin + colW*0.55
	totalW := colW * 0.45

	pdf.SetDrawColor(200, 200, 200)
	pdf.Line(totalX, pdf.GetY(), pageW-margin, pdf.GetY())
	pdf.Ln(3)

	setFont("", 9)
	pdf.SetTextColor(60, 60, 60)
	pdf.SetX(totalX)
	pdf.CellFormat(totalW*0.6, 6, "Total Hours:", "0", 0, "L", false, 0, "")
	pdf.CellFormat(totalW*0.4, 6, fmt.Sprintf("%.2fh", inv.TotalHours), "0", 1, "R", false, 0, "")

	if inv.Customer != nil && inv.Customer.HourlyRate > 0 {
		pdf.SetX(totalX)
		pdf.CellFormat(totalW*0.6, 6, "Rate:", "0", 0, "L", false, 0, "")
		pdf.CellFormat(totalW*0.4, 6, fmtMoney(inv.Customer.HourlyRate, inv.Currency)+"/h", "0", 1, "R", false, 0, "")
	}

	pdf.Ln(2)
	pdf.SetDrawColor(30, 30, 30)
	pdf.Line(totalX, pdf.GetY(), pageW-margin, pdf.GetY())
	pdf.Ln(3)

	setFont("B", 12)
	pdf.SetTextColor(30, 30, 30)
	pdf.SetX(totalX)
	pdf.CellFormat(totalW*0.6, 8, "TOTAL:", "0", 0, "L", false, 0, "")
	pdf.CellFormat(totalW*0.4, 8, fmtMoney(inv.TotalAmount, inv.Currency), "0", 1, "R", false, 0, "")

	// ── Footer ────────────────────────────────────────────────────────────
	pdf.SetY(-25)
	pdf.SetDrawColor(200, 200, 200)
	pdf.Line(margin, pdf.GetY(), pageW-margin, pdf.GetY())
	pdf.Ln(3)
	setFont("I", 8)
	pdf.SetTextColor(130, 130, 130)
	pdf.CellFormat(colW, 5, "Thank you for your business!", "0", 1, "C", false, 0, "")
	pdf.CellFormat(colW, 5, fmt.Sprintf("Generated by cash · %s", time.Now().Format("2006-01-02")), "0", 1, "C", false, 0, "")

	return outPath, pdf.OutputFileAndClose(outPath)
}

func projectRate(projectName string, inv *models.Invoice) (float64, string) {
	if inv.Customer != nil {
		return inv.Customer.HourlyRate, inv.Currency
	}
	return 0, inv.Currency
}

func fmtMoney(amount float64, currency string) string {
	symbol := currencySymbol(currency)
	return fmt.Sprintf("%s%.2f", symbol, amount)
}

func currencySymbol(currency string) string {
	switch strings.ToUpper(currency) {
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	case "CHF":
		return "CHF "
	default:
		return "$"
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
