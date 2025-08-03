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
