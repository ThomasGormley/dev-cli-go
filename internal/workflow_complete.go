package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/thomasgormley/dev-cli-go/internal/git"
	"github.com/urfave/cli/v2"
)

func handleWorkflowComplete() cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// find workflow for branch
		branch, err := git.CurrentBranch()
		if err != nil {
			return err
		}

		wf, found, err := workflowStateForBranch(branch)
		if err != nil {
			return err
		}
		if !found {
			fmt.Printf("branch '%s' does not match an active workflow\n", branch)

			if deleteOrphanedBranch := promptForDeleteBranch(branch); deleteOrphanedBranch {
				mainBranch, err := git.DetectMainBranch()
				if err != nil {
					return fmt.Errorf("failed to detect main branch: %w", err)
				}

				if err = git.Checkout(mainBranch); err != nil {
					return fmt.Errorf("failed to checkout %s branch: %w", mainBranch, err)
				}

				if err = git.DeleteLocalBranch(branch); err != nil {
					return fmt.Errorf("failed to delete %s branch: %w", branch, err)
				}

				if err = git.Pull(); err != nil {
					return fmt.Errorf("failed to pull latest changes: %w", err)
				}
			}

			return nil
		}

		// (TODO):check PR status: merged

		mainBranch, err := git.DetectMainBranch()
		if err != nil {
			return fmt.Errorf("failed to detect main branch: %w", err)
		}

		if err = git.Checkout(mainBranch); err != nil {
			return fmt.Errorf("failed to checkout %s branch: %w", mainBranch, err)
		}

		if err = git.DeleteLocalBranch(branch); err != nil {
			return fmt.Errorf("failed to delete %s branch: %w", branch, err)
		}

		if err = git.Pull(); err != nil {
			return fmt.Errorf("failed to pull latest changes: %w", err)
		}

		// remove from workflow state

		if err = deleteWorkflowState(wf.TicketID); err != nil {
			return fmt.Errorf("failed to delete workflow state: %w", err)
		}
		return nil
	}
}

func promptForDeleteBranch(branch string) bool {
	var deleteOrphanedBranch bool
	survey.AskOne(
		&survey.Confirm{Message: fmt.Sprintf("Delete '%s' branch?", branch)},
		&deleteOrphanedBranch,
	)
	return deleteOrphanedBranch
}

func workflowStateForBranch(branch string) (WorkflowEntry, bool, error) {
	wfs, err := loadWorkflowState()
	if err != nil {
		return WorkflowEntry{}, false, fmt.Errorf("loading workflow state: %s", err)
	}

	var found bool
	var wfe WorkflowEntry
	for _, w := range wfs {
		found = w.Branch == branch
		if found {
			wfe = w
			break
		}
	}

	return wfe, found, nil
}

func deleteWorkflowState(ticketID string) error {
	state, err := loadWorkflowState()
	if err != nil {
		return err
	}
	delete(state, ticketID)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	stateFile := filepath.Join(homeDir, ".dev_workflow_state.json")
	return os.WriteFile(stateFile, data, 0644)
}
