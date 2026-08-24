package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/larspittman/cash/internal/config"
	"github.com/larspittman/cash/internal/db"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or set your personal details used on invoices",
	Long: `Manage the freelancer info printed in the FROM section of invoices.

Examples:
  cash config show
  cash config set name "Lars Pittman"
  cash config set email "lars@example.com"
  cash config set phone "+351 900 000 000"
  cash config set address "123 Main St\nCity, Country"
  cash config set tax_id "123456789"
  cash config set payment_details "PT50 0018 ..."`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current config",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()
		settings, err := d.GetSettings()
		if err != nil {
			return err
		}
		cfg.ApplySettings(settings)
		fmt.Printf("Name    : %s\n", cfg.Name)
		fmt.Printf("Email   : %s\n", cfg.Email)
		fmt.Printf("Phone   : %s\n", cfg.Phone)
		fmt.Printf("Address : %s\n", cfg.Address)
		fmt.Printf("Tax ID  : %s\n", cfg.TaxID)
		fmt.Printf("Payment : %s\n", cfg.PaymentDetails)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value  (name | email | phone | address | tax_id | payment_details)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		val := args[1]

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		switch key {
		case "name":
			cfg.Name = val
		case "email":
			cfg.Email = val
		case "phone":
			cfg.Phone = val
		case "address":
			cfg.Address = val
		case "tax_id":
			cfg.TaxID = val
		case "payment_details":
			cfg.PaymentDetails = val
		default:
			return fmt.Errorf("unknown key %q: valid keys are name, email, phone, address, tax_id, payment_details", key)
		}

		if err := config.Save(cfg); err != nil {
			return err
		}
		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()
		if err := d.SetSetting(key, val); err != nil {
			return err
		}
		fmt.Printf("Config updated: %s = %q\n", key, val)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
}
