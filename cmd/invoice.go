package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/larspittman/cash/internal/config"
	"github.com/larspittman/cash/internal/db"
	"github.com/larspittman/cash/internal/models"
	"github.com/larspittman/cash/internal/pdf"
	"github.com/larspittman/cash/internal/report"
)

var invoiceCmd = &cobra.Command{
	Use:   "invoice",
	Short: "Manage and generate invoices",
}

// ── invoice generate ──────────────────────────────────────────────────────────

var (
	invCustomer string
	invFrom     string
	invTo       string
	invNumber   string
	invRate     float64
	invPaidAt   string
	invMonth    string
	invWeek     string
	invAll      bool
	invEntries  []int64
	invInclude  []string
	invExclude  []string
)

var invoiceGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a PDF invoice for a customer",
	Long: `Create an invoice from time entries in a date range or period scope.

Example:
  cash invoice generate --customer "Acme Corp" --from 2026-01-01 --to 2026-01-31
  cash invoice generate --customer "Acme Corp" --month 2026-01
  cash invoice generate --customer "Acme Corp" --month 2026-01 --include-project web_app --entry 42`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if invCustomer == "" {
			return fmt.Errorf("--customer is required")
		}
		fromTime, toTime, label, err := invoiceScope()
		if err != nil {
			return err
		}

		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		customer, err := d.GetCustomerBySlugOrName(invCustomer)
		if err != nil {
			return err
		}
		if customer == nil {
			return fmt.Errorf("customer %q not found", invCustomer)
		}

		// Override rate if provided
		if cmd.Flags().Changed("rate") {
			customer.HourlyRate = invRate
		}

		projects, err := d.GetProjectsByCustomer(customer.ID)
		if err != nil {
			return err
		}
		if len(projects) == 0 {
			fmt.Printf("Warning: no projects linked to customer %q.\n", customer.Name)
			fmt.Println("Link projects with: cash project add <name> --customer <customer>")
			return nil
		}

		selectedProjects, err := selectInvoiceProjects(projects, invInclude, invExclude)
		if err != nil {
			return err
		}
		if len(selectedProjects) == 0 {
			return fmt.Errorf("no customer projects matched the include/exclude filters")
		}

		var allEntries []models.TimeEntry
		for _, p := range selectedProjects {
			entries, err := d.GetTimeEntries(models.TimeEntryFilter{
				ProjectName: p.Name,
				From:        &fromTime,
				To:          &toTime,
			})
			if err != nil {
				return err
			}
			allEntries = append(allEntries, entries...)
		}

		var eligibleEntries []models.TimeEntry
		for _, e := range allEntries {
			if !e.Billable || e.InvoiceID != nil {
				continue
			}
			eligibleEntries = append(eligibleEntries, e)
		}

		invoiceEntries, err := selectInvoiceEntries(eligibleEntries, invEntries)
		if err != nil {
			return err
		}

		recurringEntries, err := d.GetRecurringChargeEntries(customer.ID, fromTime, toTime)
		if err != nil {
			return err
		}
		recurringEntries = filterRecurringInvoiceEntries(recurringEntries, selectedProjects)

		var eligibleRecurring []models.RecurringChargeEntry
		for _, e := range recurringEntries {
			if !e.Billable || e.InvoiceID != nil {
				continue
			}
			eligibleRecurring = append(eligibleRecurring, e)
		}

		if len(invoiceEntries) == 0 && len(eligibleRecurring) == 0 {
			fmt.Println("No uninvoiced billable entries found for this customer in the selected scope.")
			return nil
		}

		totalHours := 0.0
		invoiceEntryIDs := make([]int64, 0, len(invoiceEntries))
		for _, e := range invoiceEntries {
			totalHours += e.Hours
			invoiceEntryIDs = append(invoiceEntryIDs, e.ID)
		}
		recurringEntryIDs := make([]int64, 0, len(eligibleRecurring))
		invoiceItems := buildInvoiceItems(invoiceEntries, customer.HourlyRate, customer.Currency)
		invoiceItems = append(invoiceItems, buildRecurringInvoiceItems(eligibleRecurring)...)
		totalAmount := 0.0
		for _, item := range invoiceItems {
			totalAmount += item.Amount
		}
		for _, e := range eligibleRecurring {
			recurringEntryIDs = append(recurringEntryIDs, e.ID)
		}

		// Generate invoice number
		invoiceNum := invNumber
		if invoiceNum == "" {
			invoiceNum, err = d.NextInvoiceNumber(time.Now().Year())
			if err != nil {
				return err
			}
		}
		existingInvoice, err := d.GetInvoiceByNumber(invoiceNum)
		if err != nil {
			return err
		}
		if existingInvoice != nil {
			return fmt.Errorf("invoice %q already exists", invoiceNum)
		}

		inv := models.Invoice{
			CustomerID:    customer.ID,
			InvoiceNumber: invoiceNum,
			PeriodStart:   fromTime,
			PeriodEnd:     toTime.AddDate(0, 0, -1),
			TotalHours:    totalHours,
			TotalAmount:   totalAmount,
			Currency:      customer.Currency,
			Status:        models.InvoiceStatusDraft,
			Customer:      customer,
			Entries:       invoiceEntries,
			Items:         invoiceItems,
			CreatedAt:     time.Now(),
		}

		pdfPath, err := renderInvoicePDF(&inv)
		if err != nil {
			return fmt.Errorf("generating PDF: %w", err)
		}

		inv.PDFPath = pdfPath
		invID, err := d.CreateInvoice(inv, invoiceEntryIDs, recurringEntryIDs, invoiceItems)
		if err != nil {
			_ = os.Remove(pdfPath)
			_ = os.Remove(strings.TrimSuffix(pdfPath, filepath.Ext(pdfPath)) + ".html")
			return fmt.Errorf("saving invoice: %w", err)
		}
		inv.ID = invID

		fmt.Printf("Invoice generated  →  %s\n", invoiceNum)
		fmt.Printf("  Customer  : %s\n", customer.Name)
		fmt.Printf("  Scope     : %s\n", label)
		fmt.Printf("  Period    : %s – %s\n", fromTime.Format("Jan 2, 2006"), toTime.AddDate(0, 0, -1).Format("Jan 2, 2006"))
		fmt.Printf("  Projects  : %s\n", strings.Join(projectNames(selectedProjects), ", "))
		fmt.Printf("  Entries   : %d time entries, %d recurring items\n", len(invoiceEntries), len(eligibleRecurring))
		fmt.Printf("  Hours     : %.2fh\n", totalHours)
		if customer.HourlyRate > 0 {
			fmt.Printf("  Total     : %s\n", fmtRevenue(totalAmount, customer.Currency))
		}
		fmt.Printf("  PDF       : %s\n", pdfPath)
		return nil
	},
}

var invoicePDFCmd = &cobra.Command{
	Use:   "pdf <invoice-number>",
	Short: "Generate or re-generate the PDF for an existing invoice",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		inv, err := d.GetInvoiceByNumber(args[0])
		if err != nil {
			return err
		}
		if inv == nil {
			return fmt.Errorf("invoice %q not found", args[0])
		}

		pdfPath, err := renderInvoicePDF(inv)
		if err != nil {
			return fmt.Errorf("generating PDF: %w", err)
		}
		if err := d.UpdateInvoicePDFPath(inv.ID, pdfPath); err != nil {
			return err
		}

		fmt.Printf("Invoice PDF written  →  %s\n", pdfPath)
		return nil
	},
}

var invoiceTaxRefCmd = &cobra.Command{
	Use:   "tax-ref <invoice-number> <reference>",
	Short: "Set the Portuguese tax-system reference number for an invoice",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		inv, err := d.GetInvoiceByNumber(args[0])
		if err != nil {
			return err
		}
		if inv == nil {
			return fmt.Errorf("invoice %q not found", args[0])
		}
		if err := d.UpdateInvoiceTaxReference(inv.ID, args[1]); err != nil {
			return err
		}
		fmt.Printf("Invoice tax reference set  →  %s  (%s)\n", inv.InvoiceNumber, args[1])
		return nil
	},
}

var invoiceSendCmd = &cobra.Command{
	Use:   "send <invoice-number>",
	Short: "Mark an invoice as sent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return transitionInvoice(args[0], func(inv *models.Invoice) error {
			if inv.Status != models.InvoiceStatusDraft {
				return fmt.Errorf("only draft invoices can be sent")
			}
			return nil
		}, models.InvoiceStatusSent, time.Now(), "Invoice marked sent  →  %s\n")
	},
}

var invoicePaidCmd = &cobra.Command{
	Use:   "paid <invoice-number>",
	Short: "Mark an invoice as paid",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paidAt := time.Now()
		if strings.TrimSpace(invPaidAt) != "" {
			parsed, err := time.Parse("2006-01-02", invPaidAt)
			if err != nil {
				return fmt.Errorf("invalid --date: use YYYY-MM-DD")
			}
			paidAt = parsed
		}
		return transitionInvoice(args[0], func(inv *models.Invoice) error {
			switch inv.Status {
			case models.InvoiceStatusSent, models.InvoiceStatusPaid:
				return nil
			case models.InvoiceStatusDraft:
				return fmt.Errorf("invoice must be sent before it can be marked paid")
			case models.InvoiceStatusCancelled:
				return fmt.Errorf("cancelled invoices cannot be marked paid")
			default:
				return fmt.Errorf("unsupported invoice status %q", inv.Status)
			}
		}, models.InvoiceStatusPaid, paidAt, "Invoice marked paid  →  %s\n")
	},
}

var invoiceCancelCmd = &cobra.Command{
	Use:   "cancel <invoice-number>",
	Short: "Cancel an invoice and release its billable entries",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		inv, err := d.GetInvoiceByNumber(args[0])
		if err != nil {
			return err
		}
		if inv == nil {
			return fmt.Errorf("invoice %q not found", args[0])
		}
		switch inv.Status {
		case models.InvoiceStatusDraft, models.InvoiceStatusSent:
		case models.InvoiceStatusCancelled:
			fmt.Printf("Invoice already cancelled  →  %s\n", inv.InvoiceNumber)
			return nil
		case models.InvoiceStatusPaid:
			return fmt.Errorf("paid invoices cannot be cancelled")
		default:
			return fmt.Errorf("unsupported invoice status %q", inv.Status)
		}

		if err := d.CancelInvoice(inv.ID, time.Now()); err != nil {
			return err
		}

		fmt.Printf("Invoice cancelled  →  %s\n", inv.InvoiceNumber)
		return nil
	},
}

// ── invoice list ──────────────────────────────────────────────────────────────

var invoiceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all invoices",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		invoices, err := d.GetInvoices()
		if err != nil {
			return err
		}
		if len(invoices) == 0 {
			fmt.Println("No invoices yet.")
			return nil
		}

		fmt.Printf("%-18s  %-22s  %-22s  %8s  %8s  %s\n",
			"Number", "Customer", "Period", "Hours", "Amount", "Status")
		fmt.Println(strings.Repeat("─", 90))
		for _, inv := range invoices {
			custName := "-"
			if inv.Customer != nil {
				custName = inv.Customer.Name
			}
			period := fmt.Sprintf("%s – %s",
				inv.PeriodStart.Format("Jan 2"),
				inv.PeriodEnd.Format("Jan 2, 06"),
			)
			amount := fmtRevenue(inv.TotalAmount, inv.Currency)
			fmt.Printf("%-18s  %-22s  %-22s  %5.2fh  %8s  %s\n",
				inv.InvoiceNumber, custName, period, inv.TotalHours, amount, inv.Status)
		}
		return nil
	},
}

// ── invoice show ──────────────────────────────────────────────────────────────

var invoiceShowCmd = &cobra.Command{
	Use:   "show <invoice-number>",
	Short: "Show invoice details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		inv, err := d.GetInvoiceByNumber(args[0])
		if err != nil {
			return err
		}
		if inv == nil {
			return fmt.Errorf("invoice %q not found", args[0])
		}

		fmt.Printf("Invoice    : %s\n", inv.InvoiceNumber)
		if strings.TrimSpace(inv.TaxReference) != "" {
			fmt.Printf("Tax Ref    : %s\n", inv.TaxReference)
		}
		fmt.Printf("Status     : %s\n", inv.Status)
		if inv.Customer != nil {
			fmt.Printf("Customer   : %s\n", inv.Customer.Name)
			fmt.Printf("Email      : %s\n", inv.Customer.Email)
		}
		fmt.Printf("Period     : %s – %s\n",
			inv.PeriodStart.Format("Jan 2, 2006"),
			inv.PeriodEnd.Format("Jan 2, 2006"),
		)
		fmt.Printf("Hours      : %.2fh\n", inv.TotalHours)
		fmt.Printf("Amount     : %s\n", fmtRevenue(inv.TotalAmount, inv.Currency))
		if inv.SentAt != nil {
			fmt.Printf("Sent       : %s\n", inv.SentAt.Format("2006-01-02"))
		}
		if inv.PaidAt != nil {
			fmt.Printf("Paid       : %s\n", inv.PaidAt.Format("2006-01-02"))
		}
		if inv.CancelledAt != nil {
			fmt.Printf("Cancelled  : %s\n", inv.CancelledAt.Format("2006-01-02"))
		}
		fmt.Printf("Items      : %d\n", len(inv.Items))
		if inv.PDFPath != "" {
			fmt.Printf("PDF        : %s\n", inv.PDFPath)
		}
		fmt.Printf("Created    : %s\n", inv.CreatedAt.Format("2006-01-02"))
		return nil
	},
}

func init() {
	invoiceCmd.AddCommand(invoiceGenerateCmd)
	invoiceCmd.AddCommand(invoicePDFCmd)
	invoiceCmd.AddCommand(invoiceTaxRefCmd)
	invoiceCmd.AddCommand(invoiceSendCmd)
	invoiceCmd.AddCommand(invoicePaidCmd)
	invoiceCmd.AddCommand(invoiceCancelCmd)
	invoiceCmd.AddCommand(invoiceListCmd)
	invoiceCmd.AddCommand(invoiceShowCmd)

	invoiceGenerateCmd.Flags().StringVar(&invCustomer, "customer", "", "Customer slug or name (required)")
	invoiceGenerateCmd.Flags().StringVar(&invFrom, "from", "", "Period start date YYYY-MM-DD (used with --to)")
	invoiceGenerateCmd.Flags().StringVar(&invTo, "to", "", "Period end date YYYY-MM-DD (used with --from)")
	invoiceGenerateCmd.Flags().StringVar(&invMonth, "month", "", "Invoice scope month YYYY-MM")
	invoiceGenerateCmd.Flags().StringVar(&invWeek, "week", "", "Invoice scope week containing YYYY-MM-DD")
	invoiceGenerateCmd.Flags().StringVar(&invNumber, "number", "", "Invoice number (auto-generated if omitted)")
	invoiceGenerateCmd.Flags().Float64Var(&invRate, "rate", 0, "Override hourly rate for this invoice")
	invoiceGenerateCmd.Flags().BoolVar(&invAll, "all", false, "Include all matching uninvoiced billable entries")
	invoiceGenerateCmd.Flags().Int64SliceVar(&invEntries, "entry", nil, "Specific time entry ID to include (repeatable)")
	invoiceGenerateCmd.Flags().StringSliceVar(&invInclude, "include-project", nil, "Project names to include (repeatable)")
	invoiceGenerateCmd.Flags().StringSliceVar(&invExclude, "exclude-project", nil, "Project names to exclude (repeatable)")
	invoicePaidCmd.Flags().StringVar(&invPaidAt, "date", "", "Paid date YYYY-MM-DD (defaults to today)")
}

func invoiceScope() (time.Time, time.Time, string, error) {
	scopeCount := 0
	if strings.TrimSpace(invMonth) != "" {
		scopeCount++
	}
	if strings.TrimSpace(invWeek) != "" {
		scopeCount++
	}
	if strings.TrimSpace(invFrom) != "" || strings.TrimSpace(invTo) != "" {
		scopeCount++
	}
	if scopeCount != 1 {
		return time.Time{}, time.Time{}, "", fmt.Errorf("use exactly one scope: --month, --week, or --from with --to")
	}
	if invAll && len(invEntries) > 0 {
		return time.Time{}, time.Time{}, "", fmt.Errorf("use either --all or --entry, not both")
	}

	if strings.TrimSpace(invMonth) != "" {
		target, err := time.Parse("2006-01", invMonth)
		if err != nil {
			return time.Time{}, time.Time{}, "", fmt.Errorf("invalid --month value %q: use YYYY-MM", invMonth)
		}
		from := report.MonthStart(target)
		to := report.MonthEnd(target)
		return from, to, target.Format("January 2006"), nil
	}

	if strings.TrimSpace(invWeek) != "" {
		target, err := time.Parse("2006-01-02", invWeek)
		if err != nil {
			return time.Time{}, time.Time{}, "", fmt.Errorf("invalid --week value %q: use YYYY-MM-DD", invWeek)
		}
		from := report.WeekStart(target)
		to := from.AddDate(0, 0, 7)
		_, weekNum := from.ISOWeek()
		return from, to, fmt.Sprintf("Week %02d (%s)", weekNum, from.Format("2006-01-02")), nil
	}

	if strings.TrimSpace(invFrom) == "" || strings.TrimSpace(invTo) == "" {
		return time.Time{}, time.Time{}, "", fmt.Errorf("--from and --to are required together (format: YYYY-MM-DD)")
	}
	from, err := time.Parse("2006-01-02", invFrom)
	if err != nil {
		return time.Time{}, time.Time{}, "", fmt.Errorf("invalid --from date: use YYYY-MM-DD")
	}
	to, err := time.Parse("2006-01-02", invTo)
	if err != nil {
		return time.Time{}, time.Time{}, "", fmt.Errorf("invalid --to date: use YYYY-MM-DD")
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, "", fmt.Errorf("--to cannot be before --from")
	}
	return from, to.AddDate(0, 0, 1), fmt.Sprintf("%s to %s", from.Format("2006-01-02"), to.Format("2006-01-02")), nil
}

func selectInvoiceProjects(projects []models.Project, include, exclude []string) ([]models.Project, error) {
	includeSet := normalizeNameSet(include)
	excludeSet := normalizeNameSet(exclude)
	for name := range includeSet {
		if excludeSet[name] {
			return nil, fmt.Errorf("project %q cannot be both included and excluded", name)
		}
	}

	known := map[string]string{}
	for _, p := range projects {
		known[strings.ToLower(strings.TrimSpace(p.Name))] = p.Name
	}
	for name := range includeSet {
		if _, ok := known[name]; !ok {
			return nil, fmt.Errorf("included project %q is not linked to this customer", name)
		}
	}
	for name := range excludeSet {
		if _, ok := known[name]; !ok {
			return nil, fmt.Errorf("excluded project %q is not linked to this customer", name)
		}
	}

	selected := make([]models.Project, 0, len(projects))
	for _, p := range projects {
		key := strings.ToLower(strings.TrimSpace(p.Name))
		if len(includeSet) > 0 && !includeSet[key] {
			continue
		}
		if excludeSet[key] {
			continue
		}
		selected = append(selected, p)
	}
	return selected, nil
}

func selectInvoiceEntries(entries []models.TimeEntry, chosen []int64) ([]models.TimeEntry, error) {
	if len(chosen) == 0 {
		return entries, nil
	}

	chosenSet := map[int64]bool{}
	for _, id := range chosen {
		chosenSet[id] = true
	}

	selected := make([]models.TimeEntry, 0, len(chosenSet))
	found := map[int64]bool{}
	for _, entry := range entries {
		if chosenSet[entry.ID] {
			selected = append(selected, entry)
			found[entry.ID] = true
		}
	}

	var missing []string
	for id := range chosenSet {
		if !found[id] {
			missing = append(missing, strconv.FormatInt(id, 10))
		}
	}
	slices.Sort(missing)
	if len(missing) > 0 {
		return nil, fmt.Errorf("selected entry IDs not available for this invoice scope: %s", strings.Join(missing, ", "))
	}

	return selected, nil
}

func normalizeNameSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if key != "" {
			out[key] = true
		}
	}
	return out
}

func projectNames(projects []models.Project) []string {
	names := make([]string, 0, len(projects))
	for _, p := range projects {
		names = append(names, p.Name)
	}
	slices.Sort(names)
	return names
}

func buildInvoiceItems(entries []models.TimeEntry, rate float64, currency string) []models.InvoiceItem {
	items := make([]models.InvoiceItem, 0, len(entries))
	for _, entry := range entries {
		entryID := entry.ID
		items = append(items, models.InvoiceItem{
			TimeEntryID: &entryID,
			EntryDate:   entry.CommittedAt,
			ProjectName: entry.ProjectName,
			Description: entry.Message,
			Subservice:  entry.Subservice,
			Hours:       entry.Hours,
			Rate:        rate,
			Amount:      entry.Hours * rate,
			Currency:    currency,
		})
	}
	return items
}

func buildRecurringInvoiceItems(entries []models.RecurringChargeEntry) []models.InvoiceItem {
	items := make([]models.InvoiceItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, models.InvoiceItem{
			EntryDate:   entry.PeriodStart,
			ProjectName: entry.ProjectName,
			Description: entry.Description,
			Subservice:  entry.Subservice,
			Hours:       0,
			Rate:        0,
			Amount:      entry.Amount,
			Currency:    entry.Currency,
		})
	}
	return items
}

func filterRecurringInvoiceEntries(entries []models.RecurringChargeEntry, projects []models.Project) []models.RecurringChargeEntry {
	if len(entries) == 0 {
		return nil
	}
	allowed := make(map[string]bool, len(projects))
	for _, p := range projects {
		allowed[strings.ToLower(strings.TrimSpace(p.Name))] = true
	}
	filtered := make([]models.RecurringChargeEntry, 0, len(entries))
	for _, entry := range entries {
		if allowed[strings.ToLower(strings.TrimSpace(entry.ProjectName))] {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func renderInvoicePDF(inv *models.Invoice) (string, error) {
	dataDir, err := db.DataDir()
	if err != nil {
		return "", err
	}
	invoiceDir := filepath.Join(dataDir, "invoices")
	if err := os.MkdirAll(invoiceDir, 0o755); err != nil {
		return "", err
	}

	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	d, err := db.Open()
	if err != nil {
		return "", err
	}
	defer d.Close()
	settings, err := d.GetSettings()
	if err != nil {
		return "", err
	}
	cfg.ApplySettings(settings)
	return pdf.GenerateInvoice(inv, cfg, invoiceDir)
}

func transitionInvoice(number string, validate func(*models.Invoice) error, nextStatus string, changedAt time.Time, successFormat string) error {
	d, err := db.Open()
	if err != nil {
		return err
	}
	defer d.Close()

	inv, err := d.GetInvoiceByNumber(number)
	if err != nil {
		return err
	}
	if inv == nil {
		return fmt.Errorf("invoice %q not found", number)
	}
	if err := validate(inv); err != nil {
		return err
	}
	if err := d.UpdateInvoiceStatus(inv.ID, nextStatus, changedAt); err != nil {
		return err
	}

	fmt.Printf(successFormat, inv.InvoiceNumber)
	return nil
}
