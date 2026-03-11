package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/larspittman/cash/internal/db"
	"github.com/larspittman/cash/internal/models"
)

var (
	commitMsg        string
	commitSubservice string
	commitDate       string
)

var commitCmd = &cobra.Command{
	Use:   "commit <project> <hours>",
	Short: "Record hours spent on a project",
	Long: `Commit a block of time to a project. Hours may be decimal (e.g. 2.5).

Examples:
  cash commit web_app 3 -m "Implemented login page"
  cash commit api 1.5 -m "Fixed auth bug" -s "backend"
  cash commit api 2 -d yesterday -m "Forgot to log"
  cash commit api 1 -d 2026-03-01 -m "Backfill entry"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]
		hours, err := strconv.ParseFloat(args[1], 64)
		if err != nil || hours <= 0 {
			return fmt.Errorf("invalid hours %q: must be a positive number", args[1])
		}

		d, err := db.Open()
		if err != nil {
			return err
		}
		defer d.Close()

		now := time.Now()
		commitTime := now
		if commitDate != "" {
			switch strings.ToLower(commitDate) {
			case "yesterday":
				commitTime = now.AddDate(0, 0, -1)
			default:
				parsed, err := time.ParseInLocation("2006-01-02", commitDate, now.Location())
				if err != nil {
					return fmt.Errorf("invalid date %q: use YYYY-MM-DD or 'yesterday'", commitDate)
				}
				commitTime = parsed.Add(12 * time.Hour) // noon on that day
			}
		}
		entry := models.TimeEntry{
			ProjectName: project,
			Hours:       hours,
			Message:     commitMsg,
			Subservice:  commitSubservice,
			Billable:    true,
			CommittedAt: commitTime,
		}
		if _, err := d.AddTimeEntry(entry); err != nil {
			return fmt.Errorf("saving entry: %w", err)
		}

		fmt.Printf("[%s] %.2fh → %s\n", commitTime.Format("2006-01-02 15:04"), hours, project)
		if commitMsg != "" {
			fmt.Printf("    %s\n", commitMsg)
		}
		if commitSubservice != "" {
			fmt.Printf("    category: %s\n", commitSubservice)
		}
		return nil
	},
}

func init() {
	commitCmd.Flags().StringVarP(&commitMsg, "message", "m", "", "Description of work done")
	commitCmd.Flags().StringVarP(&commitSubservice, "subservice", "s", "", "Sub-category or service type")
	commitCmd.Flags().StringVarP(&commitDate, "date", "d", "", "Date for the entry (YYYY-MM-DD or 'yesterday')")
}
