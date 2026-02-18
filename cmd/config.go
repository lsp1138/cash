package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/larspittman/cash/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or set your personal details used on invoices",
	Long: `Manage the freelancer info printed in the FROM section of invoices.

Examples:
  cash config show
  cash config set name "Lars Pittman"
  cash config set email "lars@example.com"
  cash config set address "123 Main St\nCity, Country"`,
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
		fmt.Printf("Name    : %s\n", cfg.Name)
		fmt.Printf("Email   : %s\n", cfg.Email)
		fmt.Printf("Address : %s\n", cfg.Address)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value  (name | email | address)",
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
		case "address":
			cfg.Address = val
		default:
			return fmt.Errorf("unknown key %q: valid keys are name, email, address", key)
		}

		if err := config.Save(cfg); err != nil {
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
