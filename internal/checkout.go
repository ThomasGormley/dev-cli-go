package cli

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/thomasgormley/dev-cli-go/internal/git"
	"github.com/urfave/cli/v2"
)

func handleCheckout(stdout, stderr io.Writer) func(*cli.Context) error {
	return func(c *cli.Context) error {
		if !git.IsRepo() {
			return fmt.Errorf("not in a git repository")
		}

		branch := c.String("branch")
		if branch == "" {
			// Get list of branches sorted by most recent commit
			branches, err := listBranches()
			if err != nil {
				return fmt.Errorf("failed to list branches: %w", err)
			}

			if len(branches) == 0 {
				return fmt.Errorf("no branches found")
			}

			// Prompt user to select a branch
			prompt := &survey.Select{
				Message: "Choose a branch:",
				Options: branches,
			}

			if err := survey.AskOne(prompt, &branch); err != nil {
				return fmt.Errorf("failed to prompt for branch: %w", err)
			}

			if branch == "" {
				return fmt.Errorf("no branch selected")
			}
		}

		// Get repository root
		repoRoot, err := git.Root()
		if err != nil {
			return fmt.Errorf("failed to get repository root: %w", err)
		}

		// Get parent directory of repo
		parentDir := filepath.Dir(repoRoot)

		// Get repo name
		repoName := filepath.Base(repoRoot)

		// Create worktree path
		worktreePath := filepath.Join(parentDir, fmt.Sprintf("%s-%s", repoName, branch))

		// Check if worktree already exists
		worktrees, err := git.ListWorktrees()
		if err != nil {
			return fmt.Errorf("failed to list worktrees: %w", err)
		}

		worktreeExists := false
		for _, wt := range worktrees {
			if wt == worktreePath {
				worktreeExists = true
				break
			}
		}

		// Create worktree if it doesn't exist
		if !worktreeExists {
			fmt.Fprintf(stdout, "Creating worktree for branch '%s' at %s\n", branch, worktreePath)
			if err := git.CreateWorktree(branch, worktreePath); err != nil {
				// If branch doesn't exist, create it first
				fmt.Fprintf(stdout, "Branch '%s' does not exist, creating it from current branch...\n", branch)
				currentBranch, err := git.CurrentBranch()
				if err != nil {
					return fmt.Errorf("failed to get current branch: %w", err)
				}

				if err := exec.Command("git", "branch", branch, currentBranch).Run(); err != nil {
					return fmt.Errorf("failed to create branch: %w", err)
				}
				// Then create the worktree
				if err := git.CreateWorktree(branch, worktreePath); err != nil {
					return fmt.Errorf("failed to create worktree: %w", err)
				}
			}
		} else {
			fmt.Fprintf(stdout, "Worktree for branch '%s' already exists at %s\n", branch, worktreePath)
		}

		// Create a new tmux session for the worktree
		sessionName := fmt.Sprintf("%s-%s", repoName, branch)
		fmt.Fprintf(stdout, "Creating tmux session '%s' for worktree at %s\n", sessionName, worktreePath)

		// Check if session already exists
		tmuxCmd := exec.Command("tmux", "has-session", "-t", sessionName)
		if tmuxCmd.Run() != nil {
			// Session doesn't exist, create it
			tmuxCmd = exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-c", worktreePath)
			if err := tmuxCmd.Run(); err != nil {
				return fmt.Errorf("failed to create tmux session: %w", err)
			}
			fmt.Fprintf(stdout, "Created new tmux session '%s'\n", sessionName)
		} else {
			fmt.Fprintf(stdout, "Tmux session '%s' already exists\n", sessionName)
		}

		// Instead of attaching directly, instruct the user to run the attach command
		fmt.Fprintf(stdout, "Created tmux session '%s' for worktree at %s\n", sessionName, worktreePath)
		fmt.Fprintf(stdout, "To attach to the session, run: tmux attach-session -t %s\n", sessionName)

		return nil
	}
}

func listBranches() ([]string, error) {
	cmd := exec.Command("git", "for-each-ref", "--sort=-committerdate", "refs/heads/", "--format=%(refname:short)")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	branches := strings.Split(string(out), "\n")
	// Remove empty string at the end if present
	if len(branches) > 0 && branches[len(branches)-1] == "" {
		branches = branches[:len(branches)-1]
	}

	return branches, nil
}
