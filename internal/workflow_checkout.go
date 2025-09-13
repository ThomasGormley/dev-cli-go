package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/thomasgormley/dev-cli-go/internal/git"
	"github.com/thomasgormley/dev-cli-go/internal/linear"
	"github.com/urfave/cli/v2"
)

func handleWorkflowCheckout() cli.ActionFunc {
	token, ok := os.LookupEnv("LINEAR_API_KEY")
	if !ok {
		panic("LINEAR_API_KEY environment variable required")
	}
	client := linear.NewClient(token)

	return func(ctx *cli.Context) error {
		query := ctx.Args().First()

		// Check if query looks like a ticket ID (contains dash and numbers)
		if looksLikeTicketID(query) {
			// Load workflow state
			state, err := loadWorkflowState()
			if err != nil {
				return fmt.Errorf("failed to load workflow state: %w", err)
			}

			// Check if workflow already exists
			if _, exists := state[query]; exists {
				// Checkout existing workflow
				fmt.Printf("Checking out existing workflow for %s...\n", query)
				return handleWorkflowCheckoutForTicket(query)
			} else {
				// Start new workflow
				fmt.Printf("Starting new workflow for %s...\n", query)
				return handleWorkflowStart(client, query)
			}
		}

		return handleBranchCheckout()
	}
}

// looksLikeTicketID checks if a string resembles a ticket ID pattern
func looksLikeTicketID(s string) bool {
	// Simple heuristic: contains dash and has some numbers
	return strings.Contains(s, "-") && strings.ContainsAny(s, "0123456789")
}

func handleBranchCheckout() error {
	// Load workflow state
	state, err := loadWorkflowState()
	if err != nil {
		return fmt.Errorf("failed to load workflow state: %w", err)
	}

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

	// Checkout the branch
	err = git.Checkout(branch)
	if err != nil {
		// Check if branch exists
		branches, branchErr := git.ListBranches()
		if branchErr != nil {
			return fmt.Errorf("failed to checkout branch %s: %w", branch, err)
		}

		if branchExists := slices.Contains(branches, branch); !branchExists {
			return fmt.Errorf("branch '%s' does not exist. Available branches: %v", branch, branches)
		}

		// Branch exists but checkout failed - likely due to uncommitted changes
		return fmt.Errorf("failed to checkout branch %s (you may have uncommitted changes): %w", branch, err)
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

// handleWorkflowCheckoutForTicket handles checkout for a specific ticket
func handleWorkflowCheckoutForTicket(ticketID string) error {
	// Load workflow state
	state, err := loadWorkflowState()
	if err != nil {
		return fmt.Errorf("failed to load workflow state: %w", err)
	}

	entry, exists := state[ticketID]
	if !exists {
		return fmt.Errorf("no workflow found for ticket %s", ticketID)
	}

	// Checkout the branch
	err = git.Checkout(entry.Branch)
	if err != nil {
		// Check if branch exists
		branches, branchErr := git.ListBranches()
		if branchErr != nil {
			return fmt.Errorf("failed to checkout branch %s: %w", entry.Branch, err)
		}

		if !slices.Contains(branches, entry.Branch) {
			return fmt.Errorf("branch '%s' does not exist. Available branches: %v", entry.Branch, branches)
		}

		return fmt.Errorf("failed to checkout branch %s (you may have uncommitted changes): %w", entry.Branch, err)
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

	return nil
}

// handleWorkflowStart handles starting workflow for a specific ticket
func handleWorkflowStart(client linear.Client, ticketID string) error {
	issue, err := client.GetIssue(context.Background(), ticketID)
	if err != nil {
		return fmt.Errorf("failed to get issue %s: %w", ticketID, err)
	}

	viewer, err := client.GetViewer(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get viewer: %w", err)
	}
	err = client.AssignIssue(context.Background(), ticketID, fmt.Sprintf("%v", viewer.ID))
	if err != nil {
		return fmt.Errorf("failed to assign issue: %w", err)
	}

	branchName := string(issue.BranchName)

	// Defensive checkout
	hasChanges, err := git.HasUncommittedChanges()
	if err != nil {
		return err
	}
	didStash := false
	if hasChanges {
		stash, err := promptForStash()
		if err != nil {
			return err
		}
		if stash {
			err = git.Stash()
			if err != nil {
				return err
			}
			didStash = true
		}
	}

	mainBranch, err := git.DetectMainBranch()
	if err != nil {
		return err
	}
	err = git.Checkout(mainBranch)
	if err != nil {
		return err
	}

	err = git.Pull()
	if err != nil {
		return err
	}

	err = git.CreateBranch(branchName)
	if err != nil {
		return err
	}

	if didStash {
		pop, err := promptForPop()
		if err != nil {
			return err
		}
		if pop {
			err = git.StashPop()
			if err != nil {
				return err
			}
		}
	}

	err = storeWorkflowStatus(ticketID, branchName)
	if err != nil {
		return err
	}

	fmt.Printf("Started workflow for ticket %s on branch %s\n", ticketID, branchName)

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
