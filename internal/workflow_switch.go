package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/thomasgormley/dev-cli-go/internal/git"
	"github.com/urfave/cli/v2"
)

// fuzzyMatch performs a simple fuzzy match
func fuzzyMatch(query, target string) bool {
	query = strings.ToLower(query)
	target = strings.ToLower(target)
	queryIndex := 0
	for _, char := range target {
		if queryIndex < len(query) && byte(char) == query[queryIndex] {
			queryIndex++
		}
	}
	return queryIndex == len(query)
}

// buildWorkflowOptions builds the options list and lookup map for workflow entries and branches
func buildWorkflowOptions(state WorkflowState, branches []string) ([]string, map[string]string) {
	var options []string
	optionMap := make(map[string]string)

	// Add workflow state entries first
	for _, entry := range state {
		option := fmt.Sprintf("🎫 %s (%s)", entry.Branch, entry.TicketID)
		options = append(options, option)
		optionMap[option] = entry.Branch
	}

	// Add branches that aren't already in state
	for _, branch := range branches {
		found := false
		for _, entry := range state {
			if entry.Branch == branch {
				found = true
				break
			}
		}
		if !found {
			option := fmt.Sprintf("🌿 %s", branch)
			options = append(options, option)
			optionMap[option] = branch
		}
	}

	return options, optionMap
}

// loadWorkflowState loads the workflow state from the state file
func loadWorkflowState() (WorkflowState, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	stateFile := filepath.Join(homeDir, ".dev_workflow_state.json")

	var state WorkflowState
	if data, err := os.ReadFile(stateFile); err == nil {
		err = json.Unmarshal(data, &state)
		if err != nil {
			return nil, err
		}
	}
	return state, nil
}

func handleWorkflowSwitch() cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// Load workflow state
		state, err := loadWorkflowState()
		if err != nil {
			return fmt.Errorf("failed to load workflow state: %w", err)
		}

		// Get branches
		branches, err := git.ListBranches()
		if err != nil {
			return fmt.Errorf("failed to list branches: %w", err)
		}

		// Build options and lookup map
		options, optionMap := buildWorkflowOptions(state, branches)

		if len(options) == 0 {
			return fmt.Errorf("no branches or workflow entries found")
		}

		var selected string
		selectPrompt := &survey.Select{
			Message:  "Choose a branch:",
			Options:  options,
			Filter:   contains,
			PageSize: 16,
		}

		err = survey.AskOne(selectPrompt, &selected)
		if err != nil {
			return fmt.Errorf("failed to prompt for selection: %w", err)
		}

		branch, exists := optionMap[selected]
		if !exists {
			return fmt.Errorf("selected option not found in lookup")
		}

		// Checkout the branch
		err = git.Checkout(branch)
		if err != nil {
			return fmt.Errorf("failed to checkout branch %s: %w", branch, err)
		}

		// Pull latest
		err = git.Pull()
		if err != nil {
			fmt.Printf("Warning: failed to pull latest changes: %v\n", err)
		}

		// Find and display ticket details
		var ticketEntry *WorkflowEntry
		for _, entry := range state {
			if entry.Branch == branch {
				ticketEntry = &entry
				break
			}
		}

		if ticketEntry != nil {
			fmt.Printf("Switched to branch: %s\n", branch)
			fmt.Printf("Ticket: %s\n", ticketEntry.TicketID)
			fmt.Printf("Status: %s\n", ticketEntry.Status)
			if ticketEntry.PrURL != nil {
				fmt.Printf("PR URL: %s\n", *ticketEntry.PrURL)
			}
		} else {
			fmt.Printf("Switched to branch: %s\n", branch)
		}

		return nil
	}
}

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
			if entry.PrURL != nil {
				prURL = *entry.PrURL
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
