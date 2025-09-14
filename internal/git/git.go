package git

import (
	"bytes"
	"os/exec"
)

// IsRepo checks if the current directory is inside a git repository
func IsRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	return cmd.Run() == nil
}

// CurrentBranch returns the name of the current git branch
func CurrentBranch() (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	out, err := cmd.Output()
	return string(bytes.TrimSpace(out)), err
}

// Root returns the root directory of the git repository
func Root() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(out)), nil
}

// Add stages files or directories for commit
func Add(paths ...string) error {
	args := append([]string{"add"}, paths...)
	return exec.Command("git", args...).Run()
}

// HasUncommittedChanges checks if there are uncommitted changes in the specified paths
func HasUncommittedChanges(paths ...string) (bool, error) {
	// Check both working directory and staged changes
	args1 := append([]string{"diff", "--quiet", "--"}, paths...)
	args2 := append([]string{"diff", "--cached", "--quiet", "--"}, paths...)

	cmd1 := exec.Command("git", args1...)
	cmd2 := exec.Command("git", args2...)

	err1 := cmd1.Run()
	err2 := cmd2.Run()

	// If either command returns non-zero exit code, there are changes
	return err1 != nil || err2 != nil, nil
}

// Status returns the porcelain status output for the specified paths
func Status(paths ...string) (string, error) {
	args := append([]string{"status", "--porcelain"}, paths...)
	cmd := exec.Command("git", args...)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return out.String(), nil
}

// Commit creates a commit with the specified message
func Commit(message string) error {
	return exec.Command("git", "commit", "-m", message).Run()
}

// Push pushes changes to the specified remote and branch
func Push(remote, branch string) error {
	return exec.Command("git", "push", remote, branch).Run()
}

// ListWorktrees returns a list of all worktrees for the repository
func ListWorktrees() ([]string, error) {
	cmd := exec.Command("git", "worktree", "list")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := bytes.Split(bytes.TrimSpace(out), []byte("\n"))
	worktrees := make([]string, 0, len(lines))

	for _, line := range lines {
		fields := bytes.Fields(line)
		if len(fields) > 0 {
			worktrees = append(worktrees, string(fields[0]))
		}
	}

	return worktrees, nil
}

// CreateWorktree creates a new worktree for the specified branch at the given path
func CreateWorktree(branch, path string) error {
	cmd := exec.Command("git", "worktree", "add", path, branch)
	return cmd.Run()
}

// RemoveWorktree removes the worktree at the specified path
func RemoveWorktree(path string) error {
	cmd := exec.Command("git", "worktree", "remove", path)
	return cmd.Run()
}

// DetectMainBranch returns the name of the main branch (main or master)
func DetectMainBranch() (string, error) {
	// Check if main exists
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/main")
	if cmd.Run() == nil {
		return "main", nil
	}
	// Check if master exists
	cmd = exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/master")
	if cmd.Run() == nil {
		return "master", nil
	}
	// Default to main
	return "main", nil
}

// Checkout switches to the specified branch
func Checkout(branch string) error {
	return exec.Command("git", "checkout", branch).Run()
}

// Pull fetches and merges changes from the remote
func Pull() error {
	return exec.Command("git", "pull").Run()
}

// CreateBranch creates and switches to a new branch
func CreateBranch(branch string) error {
	return exec.Command("git", "checkout", "-b", branch).Run()
}

// Stash stashes uncommitted changes
func Stash() error {
	return exec.Command("git", "stash").Run()
}

// StashPop pops the most recent stash
func StashPop() error {
	return exec.Command("git", "stash", "pop").Run()
}

// ListBranches returns a list of all branches
func ListBranches() ([]string, error) {
	cmd := exec.Command("git", "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	branches := bytes.Split(bytes.TrimSpace(out), []byte("\n"))
	result := make([]string, 0, len(branches))
	for _, b := range branches {
		if len(b) > 0 {
			result = append(result, string(b))
		}
	}
	return result, nil
}
