package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/larspittman/cash/internal/config"
	"github.com/larspittman/cash/internal/db"
	"github.com/larspittman/cash/internal/models"
	"github.com/larspittman/cash/internal/pdf"
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
)

var invoiceGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a PDF invoice for a customer",
	Long: `Create an invoice from time entries in a date range.

Example:
  cash invoice generate --customer "Acme Corp" --from 2026-01-01 --to 2026-01-31
  cash invoice generate --customer "Acme Corp" --from 2026-01-01 --to 2026-01-31 --number INV-2026-007`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if invCustomer == "" {
			return fmt.Errorf("--customer is required")
		}
		if invFrom == "" || invTo == "" {
			return fmt.Errorf("--from and --to are required (format: YYYY-MM-DD)")
		}

		fromTime, err := time.Parse("2006-01-02", invFrom)
		if err != nil {
			return fmt.Errorf("invalid --from date: use YYYY-MM-DD")
		}
		toTime, err := time.Parse("2006-01-02", invTo)
		if err != nil {
			return fmt.Errorf("invalid --to date: use YYYY-MM-DD")
		}
		// Inclusive end date: add a day so < works correctly
		toTime = toTime.AddDate(0, 0, 1)

		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		customer, err := d.GetCustomerByName(invCustomer)
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

		// Get projects for this customer
		projects, err := d.GetProjectsByCustomer(customer.ID)
		if err != nil {
			return err
		}
		projectNames := make(map[string]bool)
		for _, p := range projects {
			projectNames[p.Name] = true
		}

		// Collect entries for all customer projects in the period
		var allEntries []models.TimeEntry
		for _, p := range projects {
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

		// Also get entries with no project link (by name matching if customer has no linked projects)
		if len(projects) == 0 {
			fmt.Printf("Warning: no projects linked to customer %q.\n", customer.Name)
			fmt.Println("Link projects with: cash project add <name> --customer <customer>")
			return nil
		}

		if len(allEntries) == 0 {
			fmt.Println("No time entries found for this customer in the specified period.")
			return nil
		}

		// Calculate totals
		totalHours := 0.0
		for _, e := range allEntries {
			totalHours += e.Hours
		}
		totalAmount := totalHours * customer.HourlyRate

		// Generate invoice number
		invoiceNum := invNumber
		if invoiceNum == "" {
			invoiceNum, err = d.NextInvoiceNumber(time.Now().Year())
			if err != nil {
				return err
			}
		}

		inv := models.Invoice{
			CustomerID:    customer.ID,
			InvoiceNumber: invoiceNum,
			PeriodStart:   fromTime,
			PeriodEnd:     toTime.AddDate(0, 0, -1),
			TotalHours:    totalHours,
			TotalAmount:   totalAmount,
			Currency:      customer.Currency,
			Status:        "draft",
			Customer:      customer,
			Entries:       allEntries,
		}

		// Save to DB
		invID, err := d.AddInvoice(inv)
		if err != nil {
			return err
		}
		inv.ID = invID

		// Prepare output directory
		dataDir, err := db.DataDir()
		if err != nil {
			return err
		}
		invoiceDir := filepath.Join(dataDir, "invoices")
		if err := os.MkdirAll(invoiceDir, 0o755); err != nil {
			return err
		}

		// Load user config
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		// Generate PDF
		pdfPath, err := pdf.GenerateInvoice(&inv, cfg, invoiceDir)
		if err != nil {
			return fmt.Errorf("generating PDF: %w", err)
		}

		if err := d.UpdateInvoicePDFPath(invID, pdfPath); err != nil {
			return err
		}

		fmt.Printf("Invoice generated  →  %s\n", invoiceNum)
		fmt.Printf("  Customer  : %s\n", customer.Name)
		fmt.Printf("  Period    : %s – %s\n", fromTime.Format("Jan 2, 2006"), toTime.AddDate(0, 0, -1).Format("Jan 2, 2006"))
		fmt.Printf("  Entries   : %d time entries\n", len(allEntries))
		fmt.Printf("  Hours     : %.2fh\n", totalHours)
		if customer.HourlyRate > 0 {
			fmt.Printf("  Total     : %s\n", fmtRevenue(totalAmount, customer.Currency))
		}
		fmt.Printf("  PDF       : %s\n", pdfPath)
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
		if inv.PDFPath != "" {
			fmt.Printf("PDF        : %s\n", inv.PDFPath)
		}
		fmt.Printf("Created    : %s\n", inv.CreatedAt.Format("2006-01-02"))
		return nil
	},
}

func init() {
	invoiceCmd.AddCommand(invoiceGenerateCmd)
	invoiceCmd.AddCommand(invoiceListCmd)
	invoiceCmd.AddCommand(invoiceShowCmd)

	invoiceGenerateCmd.Flags().StringVar(&invCustomer, "customer", "", "Customer name (required)")
	invoiceGenerateCmd.Flags().StringVar(&invFrom, "from", "", "Period start date YYYY-MM-DD (required)")
	invoiceGenerateCmd.Flags().StringVar(&invTo, "to", "", "Period end date YYYY-MM-DD (required)")
	invoiceGenerateCmd.Flags().StringVar(&invNumber, "number", "", "Invoice number (auto-generated if omitted)")
	invoiceGenerateCmd.Flags().Float64Var(&invRate, "rate", 0, "Override hourly rate for this invoice")
}
