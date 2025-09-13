package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

type WorkflowState []WorkflowEntry

// Slugify function for branch names
func slugify(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)
	// Replace spaces and non-alphanumeric with hyphens
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	s = reg.ReplaceAllString(s, "-")
	// Remove leading/trailing hyphens
	s = strings.Trim(s, "-")
	return s
}

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
			} else {
				return errors.New("aborted due to uncommitted changes")
			}
		}

		// Checkout main branch
		mainBranch, err := git.DetectMainBranch()
		if err != nil {
			return err
		}
		err = git.Checkout(mainBranch)
		if err != nil {
			return err
		}

		// Pull
		err = git.Pull()
		if err != nil {
			return err
		}

		// Checkout new branch
		err = git.CreateBranch(string(branchName))
		if err != nil {
			return err
		}

		// Pop stash if we stashed earlier
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

		// Store status
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

	var state WorkflowState
	if data, err := os.ReadFile(stateFile); err == nil {
		err = json.Unmarshal(data, &state)
		if err != nil {
			return err
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
	state = append(state, entry)

	// Write back
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateFile, data, 0644)
}
