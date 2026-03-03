package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/larspittman/cash/internal/db"
	"github.com/larspittman/cash/internal/models"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects",
	Long:  `Add, list, or show projects. Projects can be linked to customers.`,
}

// ── project add ───────────────────────────────────────────────────────────────

var (
	projCustomer string
	projRate     float64
	projDesc     string
)

var projectAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new project",
	Long: `Create a project, optionally linked to a customer.

Examples:
  cash project add web_app --customer "Acme Corp" --desc "Main website"
  cash project add internal_tool --rate 120`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		if existing, _ := d.GetProjectByName(args[0]); existing != nil {
			return fmt.Errorf("project %q already exists", args[0])
		}

		p := models.Project{
			Name:        args[0],
			Description: projDesc,
		}

		if projCustomer != "" {
			c, err := d.GetCustomerBySlugOrName(projCustomer)
			if err != nil {
				return err
			}
			if c == nil {
				return fmt.Errorf("customer %q not found", projCustomer)
			}
			p.CustomerID = &c.ID
		}

		if cmd.Flags().Changed("rate") {
			p.HourlyRate = &projRate
		}

		id, err := d.AddProject(p)
		if err != nil {
			return err
		}
		fmt.Printf("Project added  →  %s  (id %d)\n", p.Name, id)
		if projCustomer != "" {
			fmt.Printf("  customer: %s\n", projCustomer)
		}
		return nil
	},
}

// ── project list ──────────────────────────────────────────────────────────────

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		projects, err := d.GetProjects()
		if err != nil {
			return err
		}
		if len(projects) == 0 {
			fmt.Println("No projects yet. Add one with: cash project add <name>")
			return nil
		}

		fmt.Printf("%-4s  %-22s  %-22s  %8s\n", "ID", "Project", "Customer", "Rate")
		fmt.Println(strings.Repeat("─", 62))
		for _, p := range projects {
			customer := "-"
			if p.Customer != nil {
				customer = p.Customer.Name
			}
			rate := "-"
			if p.HourlyRate != nil {
				rate = fmt.Sprintf("%.2f", *p.HourlyRate)
			} else if p.Customer != nil && p.Customer.HourlyRate > 0 {
				rate = fmt.Sprintf("%.2f*", p.Customer.HourlyRate)
			}
			fmt.Printf("%-4d  %-22s  %-22s  %8s\n", p.ID, p.Name, customer, rate)
		}
		fmt.Println("  * = inherited from customer")
		return nil
	},
}

// ── project show ──────────────────────────────────────────────────────────────

var projectShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show project details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		p, err := d.GetProjectByName(args[0])
		if err != nil {
			return err
		}
		if p == nil {
			return fmt.Errorf("project %q not found", args[0])
		}

		fmt.Printf("Project     : %s\n", p.Name)
		if p.Description != "" {
			fmt.Printf("Description : %s\n", p.Description)
		}
		if p.Customer != nil {
			fmt.Printf("Customer    : %s\n", p.Customer.Name)
		} else {
			fmt.Printf("Customer    : -\n")
		}

		rate, cur := p.EffectiveRate()
		if rate > 0 {
			src := "project"
			if p.HourlyRate == nil {
				src = "customer"
			}
			fmt.Printf("Rate        : %.2f %s/h  (from %s)\n", rate, cur, src)
		} else {
			fmt.Printf("Rate        : not set\n")
		}
		fmt.Printf("Created     : %s\n", p.CreatedAt.Format("2006-01-02"))
		return nil
	},
}

func init() {
	projectCmd.AddCommand(projectAddCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectShowCmd)

	projectAddCmd.Flags().StringVar(&projCustomer, "customer", "", "Customer slug or name to link to")
	projectAddCmd.Flags().Float64Var(&projRate, "rate", 0, "Override hourly rate for this project")
	projectAddCmd.Flags().StringVar(&projDesc, "desc", "", "Project description")
}
