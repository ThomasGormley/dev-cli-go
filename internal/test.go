package cli

import (
	"context"
	"io"
	"os"
	"os/exec"

	"github.com/urfave/cli/v2"
)

func handleTest(stdout, stderr io.Writer) cli.ActionFunc {
	goTest := goTest{
		stdin:  os.Stdin,
		stdout: stdout,
		stderr: stderr,
		env:    os.Environ(),
	}
	return func(ctx *cli.Context) error {

		shouldRunAll := ctx.Bool("all")
		if shouldRunAll {
			return goTest.run(ctx.Context, "./...")
		}

		return nil
	}
}

type goTest struct {
	dir string
	env []string

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func (gt goTest) run(ctx context.Context, path string) error {
	cmd := gt.prepareCmd(ctx, path)

	return cmd.Run()
}

func (gt goTest) prepareCmd(ctx context.Context, path string, args ...string) *exec.Cmd {
	cmdArgs := append([]string{"test", path}, args...)
	cmd := exec.CommandContext(ctx, "go", cmdArgs...)
	cmd.Stdout = gt.stdout
	cmd.Stdin = gt.stdin
	cmd.Stderr = gt.stderr
	cmd.Env = gt.env

	return cmd
}
