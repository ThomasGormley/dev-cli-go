package cli

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"
)

func handleWorkflowStatus() cli.ActionFunc {
	return func(ctx *cli.Context) error {
		state, err := loadWorkflowState()
		if err != nil {
			return fmt.Errorf("failed to load workflow state: %w", err)
		}

		if len(state) == 0 {
			fmt.Println("No workflow entries found.")
			return nil
		}

		fmt.Printf("%-12s %-30s %-12s %-20s %s\n", "Ticket ID", "Branch", "Status", "PR URL", "Last Updated")
		fmt.Println(strings.Repeat("-", 100))

		for _, entry := range state {
			prURL := ""
			if entry.PrURL != "" {
				prURL = entry.PrURL
			}
			fmt.Printf("%-12s %-30s %-12s %-20s %s\n",
				entry.TicketID,
				entry.Branch,
				entry.Status,
				prURL,
				entry.LastUpdated.Format("2006-01-02 15:04"))
		}

		return nil
	}
}
