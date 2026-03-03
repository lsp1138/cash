package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/larspittman/cash/internal/db"
	"github.com/larspittman/cash/internal/models"
)

var customerCmd = &cobra.Command{
	Use:   "customer",
	Short: "Manage customers",
	Long:  `Add, list, show, or edit customers.`,
}

// ── customer add ──────────────────────────────────────────────────────────────

var (
	custEmail    string
	custAddress  string
	custRate     float64
	custCurrency string
	custSlug     string
)

var customerAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new customer",
	Long: `Create a new customer record.

Example:
  cash customer add "Acme Corp" --email billing@acme.com --address "1 Business St" --rate 150 --currency USD`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if custCurrency == "" {
			custCurrency = "USD"
		}

		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		if existing, _ := d.GetCustomerByName(args[0]); existing != nil {
			return fmt.Errorf("customer %q already exists", args[0])
		}

		c := models.Customer{
			Name:       args[0],
			Slug:       custSlug,
			Email:      custEmail,
			Address:    custAddress,
			HourlyRate: custRate,
			Currency:   custCurrency,
		}
		id, err := d.AddCustomer(c)
		if err != nil {
			return err
		}
		fmt.Printf("Customer added  →  %s  (id %d)\n", c.Name, id)
		return nil
	},
}

// ── customer list ─────────────────────────────────────────────────────────────

var customerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all customers",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		customers, err := d.GetCustomers()
		if err != nil {
			return err
		}
		if len(customers) == 0 {
			fmt.Println("No customers yet. Add one with: cash customer add <name>")
			return nil
		}

		fmt.Printf("%-4s  %-24s  %-20s  %-24s  %8s  %s\n", "ID", "Name", "Slug", "Email", "Rate", "Currency")
		fmt.Println(strings.Repeat("─", 95))
		for _, c := range customers {
			rate := "-"
			if c.HourlyRate > 0 {
				rate = fmt.Sprintf("%.2f", c.HourlyRate)
			}
			fmt.Printf("%-4d  %-24s  %-20s  %-24s  %8s  %s\n",
				c.ID, c.Name, c.Slug, c.Email, rate, c.Currency)
		}
		return nil
	},
}

// ── customer show ─────────────────────────────────────────────────────────────

var customerShowCmd = &cobra.Command{
	Use:   "show <slug-or-name>",
	Short: "Show customer details and their projects by slug or name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		c, err := d.GetCustomerBySlugOrName(args[0])
		if err != nil {
			return err
		}
		if c == nil {
			return fmt.Errorf("customer %q not found", args[0])
		}

		fmt.Printf("Customer  : %s\n", c.Name)
		fmt.Printf("Slug      : %s\n", c.Slug)
		fmt.Printf("Email     : %s\n", c.Email)
		fmt.Printf("Address   : %s\n", c.Address)
		if c.HourlyRate > 0 {
			fmt.Printf("Rate      : %.2f %s/h\n", c.HourlyRate, c.Currency)
		} else {
			fmt.Printf("Rate      : not set\n")
		}
		fmt.Printf("Since     : %s\n", c.CreatedAt.Format("2006-01-02"))

		projects, err := d.GetProjectsByCustomer(c.ID)
		if err != nil {
			return err
		}
		if len(projects) > 0 {
			fmt.Println("\nProjects:")
			for _, p := range projects {
				rate := ""
				if p.HourlyRate != nil {
					rate = fmt.Sprintf("  (%.2f/h override)", *p.HourlyRate)
				}
				fmt.Printf("  %-20s%s\n", p.Name, rate)
			}
		} else {
			fmt.Println("\nNo projects linked to this customer.")
		}
		return nil
	},
}

// ── customer edit ─────────────────────────────────────────────────────────────

var (
	editEmail    string
	editAddress  string
	editRate     float64
	editCurrency string
	editSlug     string
)

var customerEditCmd = &cobra.Command{
	Use:   "edit <slug-or-name>",
	Short: "Edit an existing customer by slug or name",
	Long: `Update a customer's details. Only provide the flags you want to change.

Example:
  cash customer edit "Acme Corp" --rate 175 --email new@acme.com`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		c, err := d.GetCustomerBySlugOrName(args[0])
		if err != nil {
			return err
		}
		if c == nil {
			return fmt.Errorf("customer %q not found", args[0])
		}

		if cmd.Flags().Changed("email") {
			c.Email = editEmail
		}
		if cmd.Flags().Changed("address") {
			c.Address = editAddress
		}
		if cmd.Flags().Changed("rate") {
			c.HourlyRate = editRate
		}
		if cmd.Flags().Changed("currency") {
			c.Currency = editCurrency
		}
		if cmd.Flags().Changed("slug") {
			c.Slug = editSlug
		}

		if err := d.UpdateCustomer(*c); err != nil {
			return err
		}
		fmt.Printf("Customer %q updated.\n", c.Name)
		return nil
	},
}

func init() {
	customerCmd.AddCommand(customerAddCmd)
	customerCmd.AddCommand(customerListCmd)
	customerCmd.AddCommand(customerShowCmd)
	customerCmd.AddCommand(customerEditCmd)

	customerAddCmd.Flags().StringVar(&custEmail, "email", "", "Billing email address")
	customerAddCmd.Flags().StringVar(&custAddress, "address", "", "Postal address")
	customerAddCmd.Flags().Float64Var(&custRate, "rate", 0, "Hourly rate")
	customerAddCmd.Flags().StringVar(&custCurrency, "currency", "USD", "Currency code (USD, EUR, GBP…)")
	customerAddCmd.Flags().StringVar(&custSlug, "slug", "", "Customer slug (default: auto-generated from name)")

	customerEditCmd.Flags().StringVar(&editEmail, "email", "", "New email")
	customerEditCmd.Flags().StringVar(&editAddress, "address", "", "New address")
	customerEditCmd.Flags().Float64Var(&editRate, "rate", 0, "New hourly rate")
	customerEditCmd.Flags().StringVar(&editCurrency, "currency", "", "New currency code")
	customerEditCmd.Flags().StringVar(&editSlug, "slug", "", "New customer slug")
}
