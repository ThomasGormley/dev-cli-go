package cli

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/thomasgormley/dev-cli-go/internal/git"
	"github.com/urfave/cli/v2"
)

func handleWorkflowComplete() cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// Get current branch
		currentBranch, err := git.CurrentBranch()
		if err != nil {
			return err
		}

		// Load workflow state
		state, err := loadWorkflowState()
		if err != nil {
			return fmt.Errorf("failed to load workflow state: %w", err)
		}

		// Build options
		options, optionMap := buildCompleteOptions(currentBranch, state)
		if len(options) == 0 {
			return fmt.Errorf("no active workflows found")
		}

		// Prompt for selection
		var selected string
		prompt := &survey.Select{
			Message:  "Choose a workflow to complete:",
			Options:  options,
			PageSize: 16,
		}
		err = survey.AskOne(prompt, &selected)
		if err != nil {
			return fmt.Errorf("failed to prompt for selection: %w", err)
		}

		branch, exists := optionMap[selected]
		if !exists {
			return fmt.Errorf("selected option not found")
		}

		// (TODO):check PR status: merged

		mainBranch, err := git.DetectMainBranch()
		if err != nil {
			return fmt.Errorf("failed to detect main branch: %w", err)
		}

		if branch == mainBranch {
			return fmt.Errorf("cannot complete main branch")
		}

		if branch == currentBranch {
			if err = git.Checkout(mainBranch); err != nil {
				return fmt.Errorf("failed to checkout %s branch: %w", mainBranch, err)
			}

			if err = git.Pull(); err != nil {
				return fmt.Errorf("failed to pull latest changes: %w", err)
			}
		}

		hasUnpushed, err := git.HasUnpushedCommits(branch)
		if err != nil {
			return fmt.Errorf("failed to check for unpushed commits: %w", err)
		}

		if hasUnpushed && !promptForDeleteBranchWithUnpushed(branch) {
			return nil
		}

		if err = git.DeleteLocalBranch(branch); err != nil {
			return fmt.Errorf("failed to delete %s branch: %w", branch, err)
		}

		// Find workflow entry
		wf, found, err := workflowStateForBranch(branch)
		if err != nil {
			return fmt.Errorf("finding workflow state for branch '%s': %v", branch, err)
		}

		if !found {
			return nil
		}

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

func promptForDeleteBranchWithUnpushed(branch string) bool {
	var deleteBranch bool
	survey.AskOne(
		&survey.Confirm{Message: fmt.Sprintf("Branch '%s' has unpushed changes. Delete anyway?", branch)},
		&deleteBranch,
	)
	return deleteBranch
}
