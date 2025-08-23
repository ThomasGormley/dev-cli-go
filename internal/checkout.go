package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

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
			return fmt.Errorf("branch name is required")
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
				// If branch doesn't exist, create it
				if err := exec.Command("git", "branch", branch).Run(); err != nil {
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

		// Change to worktree directory
		if err := os.Chdir(worktreePath); err != nil {
			return fmt.Errorf("failed to change directory to worktree: %w", err)
		}

		fmt.Fprintf(stdout, "Switched to worktree for branch '%s'\n", branch)
		return nil
	}
}
