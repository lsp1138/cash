// Package cmd implements the cash CLI commands.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cash",
	Short: "A git-style time tracking CLI for freelancers",
	Long: `cash — commit time, manage projects & customers, generate invoices.

Examples:
  cash commit my_project 2.5 -m "Frontend work" -s "UI"
  cash start my_project -m "Starting API work"
  cash stop -m "Done for today"
  cash week
  cash report month
  cash customer add "Acme Corp" --rate 150 --currency USD
  cash invoice generate --customer "Acme Corp" --from 2026-01-01 --to 2026-01-31`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(commitCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(weekCmd)
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(customerCmd)
	rootCmd.AddCommand(projectCmd)
	rootCmd.AddCommand(invoiceCmd)
	rootCmd.AddCommand(configCmd)
}
