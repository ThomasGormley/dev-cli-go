package cli

import (
	"context"
	"io"
	"os/exec"
)

type GitHubClienter interface {
	AuthStatus() (string, error)
	CreatePR(title, body, base string) (string, error)
}

type ghClient struct {
	Stderr io.Writer
	Stdout io.Writer
}

func (g *ghClient) AuthStatus() (string, error) {
	cmd := exec.Command("gh", "auth", "status")
	out, err := cmd.Output()
	return string(out), err
}

func (g *ghClient) CreatePR(title, body, base string) (string, error) {
	cmd := exec.Command("gh", "pr", "create", "--title", title, "--body", body, "--base", base)
	out, err := cmd.Output()
	return string(out), err
}

var CmdCtx = func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

func NewGitHubClient(stderr, stdout io.Writer) *ghClient {
	return &ghClient{
		Stderr: stderr,
		Stdout: stdout,
	}
}
