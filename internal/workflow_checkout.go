package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/thomasgormley/dev-cli-go/internal/git"
	"github.com/thomasgormley/dev-cli-go/internal/linear"
	"github.com/urfave/cli/v2"
)

// CHECKOUT
// - fzf all assigned tickets
//
// CHECKOUT <arg:ID>
// - checkout existing ID
// - start new id
//
// COMPLETE
// - fzf all workflow entries

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
				if err != nil {
					return fmt.Errorf("assigning issue: %v", err)
				}
				branch := string(issue.BranchName)

				entry = WorkflowEntry{
					TicketID: string(issue.Identifier),
					Branch:   branch,
					Status:   "in-progress",
				}

				if err = upsertWorkflowEntry(entry); err != nil {
					return err
				}

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

		var options []string
		optionMap := make(map[string]string)
		for _, entry := range state {
			option := fmt.Sprintf("🎫 %s (%s)", entry.Branch, entry.TicketID)
			options = append(options, option)
			optionMap[option] = entry.TicketID
		}

		if len(options) == 0 {
			return fmt.Errorf("no branches or workflow entries found")
		}

		var selectedOpt string
		prompt := &survey.Select{
			Message:  "Choose a branch:",
			Options:  options,
			PageSize: 16,
		}

		err = survey.AskOne(prompt, &selectedOpt)
		if err != nil {
			return fmt.Errorf("failed to prompt for selection: %w", err)
		}

		selectedTicketID, exists := optionMap[selectedOpt]
		if !exists {
			return fmt.Errorf("selected option not found in lookup")
		}

		selected, found, err := workflowStateForTicketID(selectedTicketID)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("ticket %s not found in entries", selectedTicketID)
		}

		err = promptForSafeCheckout(selected.Branch)
		if err != nil {
			return err
		}

		// Pull latest
		err = git.Pull()
		if err != nil {
			fmt.Printf("Warning: failed to pull latest changes: %v\n", err)
		}

		fmt.Printf("Switched to branch: %s\n", selected.Branch)
		fmt.Printf("Ticket: %s\n", selected.TicketID)
		fmt.Printf("Status: %s\n", selected.Status)
		if selected.PrURL != "" {
			fmt.Printf("PR URL: %s\n", selected.PrURL)
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
