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
		r.Fetch()
		r.Checkout(branch)
		return r, nil
	}
	os.MkdirAll(filepath.Dir(repoPath), 0755)
	cloneURL := fmt.Sprintf("git@github.com:%s/%s.git", owner, repo)
	cmd := exec.CommandContext(ctx, "git", "clone", "-b", branch, cloneURL, repoPath)
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

func (r Repo) Fetch() error {
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Dir = r.path
	return cmd.Run()
}

func (r Repo) Checkout(branch string) error {
	cmd := exec.Command("git", "checkout", branch)
	cmd.Dir = r.path
	return cmd.Run()
}

func (r Repo) ResetHard(ref string) error {
	cmd := exec.Command("git", "reset", "--hard", ref)
	cmd.Dir = r.path
	return cmd.Run()
}

func (r Repo) Status() (string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = r.path
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func (r Repo) Add(paths ...string) error {
	args := append([]string{"add"}, paths...)
	cmd := exec.Command("git", args...)
	cmd.Dir = r.path
	return cmd.Run()
}

func (r Repo) Commit(msg string) error {
	cmd := exec.Command("git", "commit", "-m", msg)
	cmd.Dir = r.path
	return cmd.Run()
}

func (r Repo) Push(remote, branch string) error {
	cmd := exec.Command("git", "push", "-u", remote, branch)
	cmd.Dir = r.path
	return cmd.Run()
}
