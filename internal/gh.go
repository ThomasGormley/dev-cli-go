package cli

import (
	"context"
	"io"
	"log"
	"os/exec"
)

type GitHubClienter interface {
	AuthStatus() error
	CreatePR(title, body, base string) error
}

type ghClient struct {
	Stderr io.Writer
	Stdout io.Writer
	Stdin  io.Reader
}

func (g *ghClient) AuthStatus() error {
	cmd := g.prepareCmd("gh", "auth", "status")
	cmd.Stdout = nil // gh auth status writes to stdout, we don't need to see it
	err := cmd.Run()
	log.Fatalf("error: %v\n", err)
	if err != nil {
		return err
	}
	return err
}

func (g *ghClient) CreatePR(title, body, base string) error {
	cmd := g.prepareCmd("gh", "pr", "create", "--title", title, "--body", body, "--base", base)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func (g *ghClient) prepareCmd(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Stdout = g.Stdout
	cmd.Stdin = g.Stdin
	cmd.Stderr = g.Stderr

	return cmd
}

var CmdCtx = func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

func NewGitHubClient(stderr, stdout io.Writer, stdin io.Reader) *ghClient {
	return &ghClient{
		Stderr: stderr,
		Stdout: stdout,
		Stdin:  stdin,
	}
}
