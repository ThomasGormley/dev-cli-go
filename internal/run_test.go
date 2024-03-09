package cli_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testing"

	cli "github.com/thomasgormley/dev-cli-go/internal"
	urfave "github.com/urfave/cli/v2"
)

func TestRunPrCreate(t *testing.T) {
	tests := map[string]struct {
		args         []string
		wantExit     int
		wantExitErr  string
		prepare      func(t *testing.T, dir string)
		ghCmdContext cli.CommandCtx
	}{
		"not a git repo": {
			args:        nil,
			wantExit:    1,
			wantExitErr: "Not a git repo",
		},
		"not authenticated": {
			args:         nil,
			wantExit:     1,
			wantExitErr:  "Not authenticated with GitHub CLI, try running `gh auth login`",
			ghCmdContext: createCommandContext(t, 1, "", ""),
			prepare: func(t *testing.T, dir string) {
				initRepo(t, dir)
			},
		},
		"with args": {
			args:         []string{"--title", "PR title", "--body", "PR body", "--base", "main"},
			wantExit:     0,
			wantExitErr:  "",
			ghCmdContext: createCommandContext(t, 0, "", ""),
			prepare: func(t *testing.T, dir string) {
				initRepo(t, dir)
			},
		},
	}

	baseArgs := []string{"dev", "pr", "create"}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			dir := t.TempDir()
			if err := os.Chdir(dir); err != nil {
				t.Fatal(err)
			}

			if tc.prepare != nil {
				tc.prepare(t, dir)
			}

			gh := cli.NewGitHubClient(stderr, stdout, tc.ghCmdContext)

			err := cli.Run(
				append(baseArgs, tc.args...),
				stdout,
				stderr,
				gh,
				func(c *urfave.Context, err error) {},
			)

			if exitErr, ok := err.(urfave.ExitCoder); ok {
				if got := exitErr.ExitCode(); got != tc.wantExit {
					t.Errorf("exit code: got %d, want %d", got, tc.wantExit)
				}

				if got := err.Error(); got != tc.wantExitErr {
					t.Errorf("exit error: got %q, want %q", got, tc.wantExitErr)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func initRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	init := exec.Command("git", "init")
	if err := init.Run(); err != nil {
		t.Fatal("error running git init:", err)
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GH_WANT_HELPER_PROCESS") != "1" {
		return
	}

	if err := func(args []string) error {
		fmt.Fprint(os.Stdout, os.Getenv("GH_HELPER_PROCESS_STDOUT"))
		exitStatus := os.Getenv("GH_HELPER_PROCESS_EXIT_STATUS")
		if exitStatus != "0" {
			return errors.New("error")
		}
		return nil
	}(os.Args[3:]); err != nil {
		if wantErr := os.Getenv("GH_HELPER_PROCESS_STDERR"); wantErr != "" {
			fmt.Fprint(os.Stderr, wantErr)
		}
		exitStatus := os.Getenv("GH_HELPER_PROCESS_EXIT_STATUS")
		i, err := strconv.Atoi(exitStatus)
		if err != nil {
			os.Exit(1)
		}
		os.Exit(i)
	}
	os.Exit(0)
}

func createCommandContext(t *testing.T, exitStatus int, stdout, stderr string) cli.CommandCtx {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=TestHelperProcess", "--")
	cmd.Env = []string{
		"GH_WANT_HELPER_PROCESS=1",
		fmt.Sprintf("GH_HELPER_PROCESS_STDOUT=%s", stdout),
		fmt.Sprintf("GH_HELPER_PROCESS_STDERR=%s", stderr),
		fmt.Sprintf("GH_HELPER_PROCESS_EXIT_STATUS=%v", exitStatus),
	}
	return func(ctx context.Context, exe string, args ...string) *exec.Cmd {
		cmd.Args = append(cmd.Args, exe)
		cmd.Args = append(cmd.Args, args...)
		return cmd
	}
}
