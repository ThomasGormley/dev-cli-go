package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/thomasgormley/dev-cli-go/internal/git"
	"github.com/thomasgormley/dev-cli-go/internal/linear"
	"github.com/urfave/cli/v2"
)

// Workflow state structures
type WorkflowEntry struct {
	TicketID     string    `json:"ticketId"`
	Branch       string    `json:"branch"`
	PrURL        *string   `json:"prUrl"`
	Status       string    `json:"status"`
	Dependencies []string  `json:"dependencies"`
	LastUpdated  time.Time `json:"lastUpdated"`
}

type WorkflowState map[string]WorkflowEntry

func handleWorkflowStart() cli.ActionFunc {
	token, ok := os.LookupEnv("LINEAR_API_KEY")
	if !ok {
		panic("linear token required")
	}
	client := linear.NewClient(token)
	return func(ctx *cli.Context) error {

		ticketID := ctx.Args().First()
		if ticketID == "" {
			return errors.New("ticketID is required")
		}

		issue, err := client.GetIssue(ctx.Context, ticketID)
		if err != nil {
			return err
		}

		// TODO optional prompt
		branchName := issue.BranchName
		// Assign to me
		viewer, err := client.GetViewer(ctx.Context)
		if err != nil {
			return fmt.Errorf("failed to get viewer: %w", err)
		}
		err = client.AssignIssue(ctx.Context, ticketID, fmt.Sprintf("%v", viewer.ID))
		if err != nil {
			return fmt.Errorf("failed to assign issue: %w", err)
		}

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

			// just allow git auto resolution
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

		err = git.CreateBranch(string(branchName))
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

		err = storeWorkflowStatus(ticketID, string(branchName))
		if err != nil {
			return err
		}

		fmt.Printf("Started workflow for ticket %s on branch %s\n", ticketID, branchName)

		return nil
	}
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
