package cli

import (
	"context"
	"io"
	"os/exec"
)

type ghClient struct {
	Stderr io.Writer
	Stdout io.Writer

	commandContext CommandCtx
}

func (g *ghClient) Run(args ...string) error {
	cmd := g.commandContext(context.Background(), "gh", args...)
	cmd.Stderr = g.Stderr
	cmd.Stdout = g.Stdout
	return cmd.Run()
}

func (g *ghClient) AuthStatus() error {
	cmd := g.commandContext(context.Background(), "gh", "auth", "status")
	return cmd.Run()
}

var CmdCtx = func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

func NewGitHubClient(stderr, stdout io.Writer, commandContext CommandCtx) *ghClient {
	if commandContext == nil {
		commandContext = exec.CommandContext
	}
	return &ghClient{
		Stderr:         stderr,
		Stdout:         stdout,
		commandContext: commandContext,
	}
}

type CommandCtx = func(ctx context.Context, name string, args ...string) *exec.Cmd
