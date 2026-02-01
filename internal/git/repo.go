package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Repo struct {
	path string
}

func EnsureClone(ctx context.Context, owner, repo, branch string) (Repo, error) {
	repoPath := filepath.Join(os.Getenv("HOME"), ".devagent/repos", owner, repo)
	if _, err := os.Stat(repoPath); err == nil {
		r := Repo{path: repoPath}
		if err := r.FetchBranch(branch); err != nil {
			return Repo{}, err
		}
		if err := r.CheckoutForce(branch); err != nil {
			return Repo{}, err
		}
		return r, nil
	}
	os.MkdirAll(filepath.Dir(repoPath), 0755)
	cloneURL := fmt.Sprintf("git@github.com:%s/%s.git", owner, repo)
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "-b", branch, cloneURL, repoPath)
	if err := cmd.Run(); err != nil {
		return Repo{}, err
	}
	return Repo{path: repoPath}, nil
}

func Open(path string) Repo {
	return Repo{path: path}
}

func (r Repo) Path() string {
	return r.path
}

func runGit(path string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = path
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %v: %w (stderr: %s)", args, err, stderr.String())
	}
	return nil
}

func (r Repo) Fetch() error {
	return runGit(r.path, "fetch", "origin")
}

func (r Repo) FetchBranch(branch string) error {
	refspec := fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", branch, branch)
	// refspec needed for shallow clone to fetch only the specific branch
	return runGit(r.path, "fetch", "origin", "--depth", "1", refspec)
}

func (r Repo) Checkout(branch string) error {
	return runGit(r.path, "checkout", branch)
}

func (r Repo) CheckoutForce(branch string) error {
	return runGit(r.path, "checkout", "-B", branch, "origin/"+branch)
}

func (r Repo) ResetHard(ref string) error {
	return runGit(r.path, "reset", "--hard", ref)
}

func (r Repo) SyncFromRemote(branch string) error {
	if err := r.FetchBranch(branch); err != nil {
		return err
	}
	return r.CheckoutForce(branch)
}

func (r Repo) Status() (string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = r.path
	var out bytes.Buffer
	cmd.Stdout = &out
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git status: %w (stderr: %s)", err, stderr.String())
	}
	return out.String(), nil
}

func (r Repo) Add(paths ...string) error {
	args := append([]string{"add"}, paths...)
	return runGit(r.path, args...)
}

func (r Repo) Commit(msg string) error {
	return runGit(r.path, "commit", "-m", msg)
}

func (r Repo) Push(remote, branch string) error {
	return runGit(r.path, "push", "-u", remote, branch)
}
