package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/thomasgormley/dev-cli-go/internal/git"
	"github.com/thomasgormley/dev-cli-go/internal/linear"
	"github.com/urfave/cli/v2"
)

func handleWorkflowCheckout() cli.ActionFunc {
	apiKey, foundKey := os.LookupEnv("LINEAR_API_KEY")
	if !foundKey {
		fmt.Println("WARNING: LINEAR_API_KEY not set")
	}
	client := linear.NewClient(apiKey)

	return func(ctx *cli.Context) error {
		if !foundKey || apiKey == "" {
			return errors.New("LINEAR_API_KEY missing")
		}

		if arg := ctx.Args().First(); looksLikeTicketID(arg) {
			// Attempt to checkout an existing workflow
			entry, existingWorkflow, err := workflowStateForTicketID(arg)
			if err != nil {
				return err
			}

			if !existingWorkflow {
				issue, err := assignIssue(client, arg)
				branch := string(issue.BranchName)

				err = promptForSafeCheckout(branch)
				if err != nil {
					return fmt.Errorf("failed to checkout: %+v", err)
				}

				err = storeWorkflowStatus(arg, branch)
				if err != nil {
					return err
				}
				fmt.Printf("Started workflow for ticket %s on branch %s\n", arg, branch)
				return nil
			}

			err = promptForSafeCheckout(entry.Branch)
			if err != nil {
				return err
			}

			// Pull latest
			err = git.Pull()
			if err != nil {
				fmt.Printf("Warning: failed to pull latest changes: %v\n", err)
			}

			fmt.Printf("Switched to branch: %s\n", entry.Branch)
			fmt.Printf("Ticket: %s\n", entry.TicketID)
			fmt.Printf("Status: %s\n", entry.Status)
			if entry.PrURL != "" {
				fmt.Printf("PR URL: %s\n", entry.PrURL)
			}

		}

		state, err := loadWorkflowState()
		if err != nil {
			return fmt.Errorf("failed to load workflow state: %w", err)
		}

		// Build options and lookup map
		options, optionMap := buildWorkflowOptions(state)

		if len(options) == 0 {
			return fmt.Errorf("no branches or workflow entries found")
		}

		var selected string
		prompt := &survey.Select{
			Message:  "Choose a branch:",
			Options:  options,
			PageSize: 16,
		}

		err = survey.AskOne(prompt, &selected)
		if err != nil {
			return fmt.Errorf("failed to prompt for selection: %w", err)
		}

		branch, exists := optionMap[selected]
		if !exists {
			return fmt.Errorf("selected option not found in lookup")
		}

		err = promptForSafeCheckout(branch)
		if err != nil {
			return err
		}

		// Pull latest
		err = git.Pull()
		if err != nil {
			fmt.Printf("Warning: failed to pull latest changes: %v\n", err)
		}

		// Find and display ticket details
		var ticketEntry WorkflowEntry
		found := false
		for _, entry := range state {
			if entry.Branch == branch {
				ticketEntry = entry
				found = true
				break
			}
		}

		if found {
			fmt.Printf("Switched to branch: %s\n", branch)
			fmt.Printf("Ticket: %s\n", ticketEntry.TicketID)
			fmt.Printf("Status: %s\n", ticketEntry.Status)
			if ticketEntry.PrURL != "" {
				fmt.Printf("PR URL: %s\n", ticketEntry.PrURL)
			}
		} else {
			fmt.Printf("Switched to branch: %s\n", branch)
		}

		return nil
	}
}

func assignIssue(client linear.Client, query string) (linear.Issue, error) {
	issue, err := client.GetIssue(context.Background(), query)
	if err != nil {
		return linear.Issue{}, fmt.Errorf("failed to get issue %s: %w", query, err)
	}

	viewer, err := client.GetViewer(context.Background())
	if err != nil {
		return linear.Issue{}, fmt.Errorf("failed to get viewer: %w", err)
	}
	err = client.AssignIssue(context.Background(), query, fmt.Sprintf("%v", viewer.ID))
	if err != nil {
		return linear.Issue{}, fmt.Errorf("failed to assign issue: %w", err)
	}
	return issue, err
}

// looksLikeTicketID checks if a string resembles a ticket ID pattern
func looksLikeTicketID(s string) bool {
	// Simple heuristic: contains dash and has some numbers
	return strings.Contains(s, "-") && strings.ContainsAny(s, "0123456789")
}

func workflowStateForTicketID(id string) (WorkflowEntry, bool, error) {
	// Load workflow state
	state, err := loadWorkflowState()
	if err != nil {
		return WorkflowEntry{}, false, fmt.Errorf("failed to load workflow state: %w", err)
	}

	entry, exists := state[id]
	if !exists {
		return WorkflowEntry{}, false, nil
	}

	return entry, true, nil
}

func promptForSafeCheckout(branchName string) error {
	hasChanges, err := git.HasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("failed to check for uncommitted changes: %w", err)
	}
	didStash := false
	if hasChanges {
		stash, err := promptForStash()
		if err != nil {
			return fmt.Errorf("failed to prompt for stashing changes: %w", err)
		}
		if stash {
			err = git.Stash()
			if err != nil {
				return fmt.Errorf("failed to stash changes: %w", err)
			}
			didStash = true
		}
	}

	mainBranch, err := git.DetectMainBranch()
	if err != nil {
		return fmt.Errorf("failed to detect main branch: %w", err)
	}
	err = git.Checkout(mainBranch)
	if err != nil {
		return fmt.Errorf("failed to checkout %s branch: %w", mainBranch, err)
	}

	err = git.Pull()
	if err != nil {
		return fmt.Errorf("failed to pull latest changes: %w", err)
	}

	err = git.CreateBranch(branchName)
	if err != nil {
		return fmt.Errorf("failed to create branch %s: %w", branchName, err)
	}

	if didStash {
		pop, err := promptForPop()
		if err != nil {
			return fmt.Errorf("failed to prompt for popping stash: %w", err)
		}
		if pop {
			err = git.StashPop()
			if err != nil {
				return fmt.Errorf("failed to pop stashed changes: %w", err)
			}
		}
	}

	return nil
}

// Workflow state structures
type WorkflowEntry struct {
	TicketID     string    `json:"ticketId"`
	Branch       string    `json:"branch"`
	PrURL        string    `json:"prUrl"`
	Status       string    `json:"status"`
	Dependencies []string  `json:"dependencies"`
	LastUpdated  time.Time `json:"lastUpdated"`
}

type WorkflowState map[string]WorkflowEntry

// buildWorkflowOptions builds the options list and lookup map for workflow entries and branches
func buildWorkflowOptions(state WorkflowState) ([]string, map[string]string) {
	var options []string
	optionMap := make(map[string]string)

	// Add workflow state entries first
	for _, entry := range state {
		option := fmt.Sprintf("🎫 %s (%s)", entry.Branch, entry.TicketID)
		options = append(options, option)
		optionMap[option] = entry.Branch
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

	state := make(WorkflowState)
	if data, err := os.ReadFile(stateFile); err == nil {
		// Try to unmarshal as map first (new format)
		err = json.Unmarshal(data, &state)
		if err != nil {
			// If that fails, try to unmarshal as array (old format) for backward compatibility
			var oldState []WorkflowEntry
			if err := json.Unmarshal(data, &oldState); err != nil {
				return nil, err
			}
			// Convert old format to new format
			state = make(WorkflowState)
			for _, entry := range oldState {
				state[entry.TicketID] = entry
			}
		}
	}
	return state, nil
}

func promptForStash() (bool, error) {
	var stash bool
	prompt := &survey.Confirm{
		Message: "You have uncommitted changes. Stash them?",
	}
	err := survey.AskOne(prompt, &stash)
	return stash, err
}

func promptForPop() (bool, error) {
	var pop bool
	prompt := &survey.Confirm{
		Message: "Pop the stashed changes onto the new branch?",
	}
	err := survey.AskOne(prompt, &pop)
	return pop, err
}

func storeWorkflowStatus(ticketID, branchName string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	stateFile := filepath.Join(homeDir, ".dev_workflow_state.json")

	state := make(WorkflowState)
	if data, err := os.ReadFile(stateFile); err == nil {
		// Try to unmarshal as map first (new format)
		err = json.Unmarshal(data, &state)
		if err != nil {
			// If that fails, try to unmarshal as array (old format) for backward compatibility
			var oldState []WorkflowEntry
			if err := json.Unmarshal(data, &oldState); err != nil {
				return err
			}
			// Convert old format to new format
			state = make(WorkflowState)
			for _, entry := range oldState {
				state[entry.TicketID] = entry
			}
		}
	}

	// Add new entry
	entry := WorkflowEntry{
		TicketID:     ticketID,
		Branch:       branchName,
		Status:       "in-progress",
		Dependencies: []string{},
		LastUpdated:  time.Now(),
	}
	state[ticketID] = entry

	// Write back (always in new map format)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateFile, data, 0644)
}
