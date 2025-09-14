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

		var (
			selectedWorkflow WorkflowEntry
			err              error
		)

		if arg := ctx.Args().First(); looksLikeTicketID(arg) {
			// Attempt to checkout an existing workflow
			selectedWorkflow, existingWorkflow, err := workflowStateForTicketID(arg)
			if err != nil {
				return err
			}

			if !existingWorkflow {
				issue, err := assignIssue(client, arg)
				if err != nil {
					return fmt.Errorf("assigning issue: %v", err)
				}
				branch := string(issue.BranchName)

				selectedWorkflow = WorkflowEntry{
					TicketID: string(issue.Identifier),
					Branch:   branch,
					Status:   "in-progress",
				}

				if err = upsertWorkflowEntry(selectedWorkflow); err != nil {
					return err
				}
			}
			return nil
		} else {
			selectedWorkflow, err = promptForWorkflowEntry()
			if err != nil {
				return err
			}
		}

		if err = safeCheckout(selectedWorkflow.Branch); err != nil {
			return err
		}

		fmt.Printf("Switched to branch: %s\n", selectedWorkflow.Branch)
		fmt.Printf("Ticket: %s\n", selectedWorkflow.TicketID)
		fmt.Printf("Status: %s\n", selectedWorkflow.Status)
		if selectedWorkflow.PrURL != "" {
			fmt.Printf("PR URL: %s\n", selectedWorkflow.PrURL)
		}

		return nil
	}
}

func promptForWorkflowEntry() (WorkflowEntry, error) {
	state, err := loadWorkflowState()
	if err != nil {
		return WorkflowEntry{}, fmt.Errorf("failed to load workflow state: %w", err)
	}

	var options []string
	optionMap := make(map[string]string)
	for _, entry := range state {
		option := fmt.Sprintf("🎫 %s (%s)", entry.Branch, entry.TicketID)
		options = append(options, option)
		optionMap[option] = entry.TicketID
	}

	if len(options) == 0 {
		return WorkflowEntry{}, fmt.Errorf("no branches or workflow entries found")
	}

	var selectedOpt string
	prompt := &survey.Select{
		Message:  "Choose a branch:",
		Options:  options,
		PageSize: 16,
	}

	err = survey.AskOne(prompt, &selectedOpt)
	if err != nil {
		return WorkflowEntry{}, fmt.Errorf("failed to prompt for selection: %w", err)
	}

	selectedTicketID, exists := optionMap[selectedOpt]
	if !exists {
		return WorkflowEntry{}, fmt.Errorf("selected option not found in lookup")
	}

	selected, found, err := workflowStateForTicketID(selectedTicketID)
	if err != nil {
		return WorkflowEntry{}, err
	}
	if !found {
		return WorkflowEntry{}, fmt.Errorf("ticket %s not found in entries", selectedTicketID)
	}
	return selected, err
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

func safeCheckout(branchName string) error {
	hasChanges, err := git.HasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("failed to check for uncommitted changes: %w", err)
	}
	didStash := false
	if hasChanges {
		if err = git.Stash(); err != nil {
			return fmt.Errorf("failed to stash changes: %w", err)
		}
		didStash = true
	}

	mainBranch, err := git.DetectMainBranch()
	if err != nil {
		return fmt.Errorf("failed to detect main branch: %w", err)
	}

	if err = git.Checkout(mainBranch); err != nil {
		return fmt.Errorf("failed to checkout %s branch: %w", mainBranch, err)
	}
	if err = git.Pull(); err != nil {
		return fmt.Errorf("failed to pull latest changes: %w", err)
	}
	if err = git.CreateBranch(branchName); err != nil {
		return fmt.Errorf("failed to create branch %s: %w", branchName, err)
	}

	if didStash {
		if err = git.StashPop(); err != nil {
			return fmt.Errorf("failed to pop stashed changes: %w", err)
		}
	}

	return nil
}
