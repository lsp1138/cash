// Package pdf renders invoice HTML/CSS and prints it to PDF.
package pdf

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/larspittman/cash/internal/config"
	"github.com/larspittman/cash/internal/models"
)

var browserCandidates = []string{
	"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	"/Applications/Chromium.app/Contents/MacOS/Chromium",
	"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
	"brave-browser",
	"brave",
	"google-chrome",
	"google-chrome-stable",
	"chromium",
	"chromium-browser",
	"microsoft-edge",
}

var invoiceTemplate = template.Must(template.New("invoice").Funcs(template.FuncMap{
	"money": func(amount float64, currency string) string {
		return fmtMoney(amount, currency)
	},
}).Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>{{ .InvoiceNumber }}</title>
  <style>
    @page {
      size: A4;
      margin: 0;
    }
    :root {
      --ink: #1b1b18;
      --muted: #6c695f;
      --line: #dbd5ca;
      --soft: #f6f1e8;
      --accent: #1f4f46;
      --accent-soft: #dfece8;
    }
    * { box-sizing: border-box; }
    html, body { margin: 0; padding: 0; }
    body {
      font-family: "Avenir Next", "Helvetica Neue", Helvetica, Arial, sans-serif;
      color: var(--ink);
      background: #fcfbf7;
      padding: 0;
    }
    .sheet {
      width: 210mm;
      min-height: 297mm;
      margin: 0 auto;
      background: #fffdfa;
      padding: 13mm 14mm 10mm;
      position: relative;
      overflow: hidden;
    }
    .sheet::before {
      content: "";
      position: absolute;
      inset: 0 0 auto 0;
      height: 6mm;
      background:
        radial-gradient(circle at 18% 40%, rgba(31, 79, 70, 0.18), transparent 34%),
        linear-gradient(90deg, rgba(31, 79, 70, 0.95), rgba(40, 108, 96, 0.78));
    }
    .topbar {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      padding-top: 1.5mm;
      position: relative;
      z-index: 1;
    }
    .brand h1 {
      margin: 0;
      font-size: 24px;
      letter-spacing: 0.16em;
      color: var(--accent);
    }
    .meta {
      min-width: 55mm;
      background: var(--soft);
      border: 1px solid var(--line);
      border-radius: 12px;
      padding: 10px 12px;
    }
    .meta-row {
      display: flex;
      justify-content: space-between;
      gap: 10px;
      padding: 3px 0;
      font-size: 11px;
    }
    .meta-row span:first-child {
      color: var(--muted);
      text-transform: uppercase;
      letter-spacing: 0.08em;
      font-size: 9px;
    }
    .parties {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 10mm;
      margin-top: 6mm;
    }
    .party {
      padding-top: 5px;
      border-top: 1px solid var(--line);
    }
    .party-label {
      color: var(--muted);
      text-transform: uppercase;
      letter-spacing: 0.12em;
      font-size: 9px;
      margin-bottom: 6px;
    }
    .party-name {
      font-weight: 700;
      font-size: 14px;
      margin-bottom: 3px;
    }
    .party-line {
      color: #3d3a34;
      font-size: 11px;
      line-height: 1.3;
      white-space: pre-line;
    }
    .summary {
      margin-top: 5mm;
      display: flex;
      justify-content: flex-end;
      align-items: start;
    }
    .summary-total {
      background: #fcfaf4;
      border: 1px solid var(--line);
      border-radius: 14px;
      padding: 10px 12px;
    }
    .summary-total .label {
      color: var(--muted);
      text-transform: uppercase;
      letter-spacing: 0.12em;
      font-size: 9px;
    }
    .summary-total .amount {
      margin-top: 6px;
      font-size: 24px;
      font-weight: 700;
      color: var(--accent);
    }
    .summary-total .hours {
      margin-top: 4px;
      font-size: 11px;
      color: var(--muted);
    }
    .groups {
      margin-top: 5mm;
    }
    .group {
      margin-bottom: 4mm;
      page-break-inside: avoid;
    }
    .group-header {
      display: flex;
      align-items: baseline;
      border-bottom: 2px solid var(--accent-soft);
      padding-bottom: 4px;
      margin-bottom: 3px;
    }
    .group-title {
      font-size: 14px;
      font-weight: 700;
      color: var(--accent);
    }
    table {
      width: 100%;
      border-collapse: collapse;
      font-size: 9px;
    }
    thead th {
      text-align: left;
      color: var(--muted);
      font-size: 9px;
      text-transform: uppercase;
      letter-spacing: 0.08em;
      padding: 0 0 6px;
      border-bottom: 1px solid var(--line);
    }
    tbody td {
      padding: 4px 0;
      border-bottom: 1px solid rgba(219,213,202,0.55);
      vertical-align: top;
    }
    tbody tr:last-child td {
      border-bottom: none;
    }
    tfoot td {
      padding: 5px 0 0;
      border-top: 1px solid var(--line);
      font-size: 9px;
    }
    td.num, th.num { text-align: right; }
    .desc {
      font-weight: 600;
      color: var(--ink);
    }
    .sub {
      margin-top: 2px;
      color: var(--muted);
      font-size: 9px;
    }
    .subtotal-label {
      color: var(--muted);
      font-weight: 600;
    }
    .footer {
      margin-top: 4mm;
      padding-top: 3mm;
      border-top: 1px solid var(--line);
      color: var(--muted);
      font-size: 9px;
    }
    .footer-top {
      display: flex;
      justify-content: space-between;
      gap: 16px;
      margin-bottom: 2.5mm;
    }
    .payment-details {
      font-size: 8px;
      line-height: 1.25;
      white-space: pre-line;
    }
    .payment-details strong {
      font-size: 8px;
    }
  </style>
</head>
<body>
  <div class="sheet">
    <div class="topbar">
      <div class="brand">
        <h1>INVOICE</h1>
      </div>
      <div class="meta">
        <div class="meta-row"><span>Invoice</span><strong>{{ .InvoiceNumber }}</strong></div>
        {{- if .TaxInvoiceNumber }}
        <div class="meta-row"><span>Tax Ref</span><strong>{{ .TaxInvoiceNumber }}</strong></div>
        {{- end }}
        <div class="meta-row"><span>Date</span><strong>{{ .InvoiceDate }}</strong></div>
        <div class="meta-row"><span>Due Date</span><strong>{{ .DueDate }}</strong></div>
        <div class="meta-row"><span>Period</span><strong>{{ .Period }}</strong></div>
        <div class="meta-row"><span>Status</span><strong>{{ .Status }}</strong></div>
      </div>
    </div>

    <div class="parties">
      <div class="party">
        <div class="party-label">From</div>
        <div class="party-name">{{ .FromName }}</div>
        {{- if .FromEmail }}<div class="party-line">{{ .FromEmail }}</div>{{ end }}
        {{- if .FromPhone }}<div class="party-line">{{ .FromPhone }}</div>{{ end }}
        {{- if .FromAddress }}<div class="party-line">{{ .FromAddress }}</div>{{ end }}
        {{- if .TaxID }}<div class="party-line">NIF: {{ .TaxID }}</div>{{ end }}
      </div>
      <div class="party">
        <div class="party-label">Bill To</div>
        <div class="party-name">{{ .ToName }}</div>
        {{- if .ToEmail }}<div class="party-line">{{ .ToEmail }}</div>{{ end }}
        {{- if .ToAddress }}<div class="party-line">{{ .ToAddress }}</div>{{ end }}
      </div>
    </div>

    <div class="summary">
      <div class="summary-total">
        <div class="label">Invoice Total (excl. VAT)</div>
        <div class="amount">{{ money .TotalAmount .Currency }}</div>
        <div class="hours">{{ printf "%.2f" .TotalHours }} hours total</div>
      </div>
    </div>

    <div class="groups">
      {{- range .Groups }}
      <section class="group">
        <div class="group-header">
          <div class="group-title">{{ .ProjectName }}</div>
        </div>
        <table>
          <thead>
            <tr>
              <th style="width: 15%;">Date</th>
              <th style="width: 43%;">Description</th>
              <th style="width: 12%;">Service</th>
              <th class="num" style="width: 8%;">Hours</th>
              <th class="num" style="width: 11%;">Rate<br>(excl. VAT)</th>
              <th class="num" style="width: 11%;">Amount<br>(excl. VAT)</th>
            </tr>
          </thead>
          <tbody>
            {{- range .Items }}
            <tr>
              <td>{{ .Date }}</td>
              <td>
                <div class="desc">{{ .Description }}</div>
                {{- if .Subnote }}<div class="sub">{{ .Subnote }}</div>{{ end }}
              </td>
              <td>{{ .Service }}</td>
              <td class="num">{{ .Hours }}</td>
              <td class="num">{{ .Rate }}</td>
              <td class="num">{{ .Amount }}</td>
            </tr>
            {{- end }}
          </tbody>
          <tfoot>
            <tr>
              <td colspan="3" class="subtotal-label">Project subtotal (excl. VAT)</td>
              <td class="num"><strong>{{ printf "%.2f" .Hours }}</strong></td>
              <td></td>
              <td class="num"><strong>{{ money .Amount .Currency }}</strong></td>
            </tr>
          </tfoot>
        </table>
      </section>
      {{- end }}
    </div>

    <div class="footer">
      <div class="footer-top">
        <div>Thank you for your business.</div>
        <div>Generated by cash · {{ .InvoiceDate }}</div>
      </div>
      {{- if .PaymentInfo }}
      <div class="payment-details"><strong>Payment details</strong><br>{{ .PaymentInfo }}</div>
      {{- end }}
    </div>
  </div>
</body>
</html>`))

type invoiceTemplateData struct {
	InvoiceNumber    string
	TaxInvoiceNumber string
	InvoiceDate      string
	DueDate          string
	Period           string
	Status           string
	FromName         string
	FromEmail        string
	FromPhone        string
	FromAddress      string
	TaxID            string
	PaymentInfo      string
	ToName           string
	ToEmail          string
	ToAddress        string
	TotalHours       float64
	TotalAmount      float64
	Currency         string
	Groups           []projectGroup
}

type projectGroup struct {
	ProjectName string
	Hours       float64
	Amount      float64
	Currency    string
	Items       []projectItem
}

type projectItem struct {
	Date        string
	Description string
	Subnote     string
	Service     string
	Hours       string
	Rate        string
	Amount      string
}

// GenerateInvoice creates invoice HTML and prints it to a PDF in outputDir.
func GenerateInvoice(inv *models.Invoice, cfg config.Config, outputDir string) (string, error) {
	filename := fmt.Sprintf("%s.pdf", inv.InvoiceNumber)
	htmlFilename := fmt.Sprintf("%s.html", inv.InvoiceNumber)
	outPath := filepath.Join(outputDir, filename)
	htmlPath := filepath.Join(outputDir, htmlFilename)

	data := buildTemplateData(inv, cfg)
	var html bytes.Buffer
	if err := invoiceTemplate.Execute(&html, data); err != nil {
		return "", fmt.Errorf("rendering invoice template: %w", err)
	}
	if err := os.WriteFile(htmlPath, html.Bytes(), 0o644); err != nil {
		return "", fmt.Errorf("writing invoice html: %w", err)
	}
	cleanupArtifacts := func() {
		_ = os.Remove(outPath)
		_ = os.Remove(htmlPath)
	}

	browser, err := findBrowser()
	if err != nil {
		cleanupArtifacts()
		return "", err
	}

	absHTML, err := filepath.Abs(htmlPath)
	if err != nil {
		cleanupArtifacts()
		return "", err
	}
	absPDF, err := filepath.Abs(outPath)
	if err != nil {
		cleanupArtifacts()
		return "", err
	}
	htmlURL := url.URL{Scheme: "file", Path: absHTML}

	cmd := exec.Command(
		browser,
		"--headless",
		"--disable-gpu",
		"--no-first-run",
		"--no-default-browser-check",
		"--allow-file-access-from-files",
		"--print-to-pdf="+absPDF,
		"--virtual-time-budget=1500",
		htmlURL.String(),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		cleanupArtifacts()
		return "", fmt.Errorf("printing html to pdf with %s: %w\n%s", browser, err, strings.TrimSpace(string(output)))
	}

	return outPath, nil
}

func buildTemplateData(inv *models.Invoice, cfg config.Config) invoiceTemplateData {
	order := []string{}
	groups := map[string]*projectGroup{}

	for _, item := range inv.Items {
		group, ok := groups[item.ProjectName]
		if !ok {
			group = &projectGroup{
				ProjectName: item.ProjectName,
				Currency:    item.Currency,
			}
			groups[item.ProjectName] = group
			order = append(order, item.ProjectName)
		}
		group.Hours += item.Hours
		group.Amount += item.Amount
		group.Items = append(group.Items, projectItem{
			Date:        item.EntryDate.Format("Jan 02"),
			Description: fallback(item.Description, "-"),
			Subnote:     strings.TrimSpace(item.Subservice),
			Service:     fallback(item.Subservice, "-"),
			Hours:       fmt.Sprintf("%.2f", item.Hours),
			Rate:        fmtMoney(item.Rate, item.Currency),
			Amount:      fmtMoney(item.Amount, item.Currency),
		})
	}

	orderedGroups := make([]projectGroup, 0, len(order))
	for _, name := range order {
		orderedGroups = append(orderedGroups, *groups[name])
	}

	status := strings.Title(inv.Status)
	if status == "" {
		status = "Draft"
	}

	return invoiceTemplateData{
		InvoiceNumber:    inv.InvoiceNumber,
		TaxInvoiceNumber: strings.TrimSpace(inv.TaxReference),
		InvoiceDate:      invoiceDate(inv).Format("January 2, 2006"),
		DueDate:          invoiceDate(inv).AddDate(0, 0, 7).Format("January 2, 2006"),
		Period:           fmt.Sprintf("%s – %s", inv.PeriodStart.Format("Jan 2, 2006"), inv.PeriodEnd.Format("Jan 2, 2006")),
		Status:           status,
		FromName:         fallback(cfg.Name, "Freelancer"),
		FromEmail:        strings.TrimSpace(cfg.Email),
		FromPhone:        strings.TrimSpace(cfg.Phone),
		FromAddress:      strings.TrimSpace(cfg.Address),
		TaxID:            strings.TrimSpace(cfg.TaxID),
		PaymentInfo:      strings.TrimSpace(cfg.PaymentDetails),
		ToName:           customerName(inv),
		ToEmail:          customerEmail(inv),
		ToAddress:        customerAddress(inv),
		TotalHours:       inv.TotalHours,
		TotalAmount:      inv.TotalAmount,
		Currency:         inv.Currency,
		Groups:           orderedGroups,
	}
}

func findBrowser() (string, error) {
	if path := strings.TrimSpace(os.Getenv("CASH_PDF_BROWSER")); path != "" {
		return path, nil
	}
	for _, candidate := range browserCandidates {
		if filepath.IsAbs(candidate) {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
			continue
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no Chromium-based browser found for invoice PDF rendering; set CASH_PDF_BROWSER to a Brave/Chrome/Chromium binary")
}

func invoiceDate(inv *models.Invoice) time.Time {
	if inv.SentAt != nil {
		return *inv.SentAt
	}
	if !inv.CreatedAt.IsZero() {
		return inv.CreatedAt
	}
	return time.Now()
}

func customerName(inv *models.Invoice) string {
	if inv.Customer != nil && strings.TrimSpace(inv.Customer.Name) != "" {
		return inv.Customer.Name
	}
	return "Customer"
}

func customerEmail(inv *models.Invoice) string {
	if inv.Customer != nil {
		return strings.TrimSpace(inv.Customer.Email)
	}
	return ""
}

func customerAddress(inv *models.Invoice) string {
	if inv.Customer != nil {
		return strings.TrimSpace(inv.Customer.Address)
	}
	return ""
}

func fallback(value, def string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return def
	}
	return value
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
