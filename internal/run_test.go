package cli_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"testing"

	cli "github.com/thomasgormley/dev-cli-go/internal"
	urfave "github.com/urfave/cli/v2"
)

type mockGitHubClient struct {
	authStatusFunc func() error
	createPRFunc   func(title, body, base string) error
}

func (m *mockGitHubClient) AuthStatus() error {
	return m.authStatusFunc()
}

func (m *mockGitHubClient) CreatePR(title, body, base string) error {
	fmt.Println("Creating PR with test args title:", title, "body:", body, "base:", base)
	return m.createPRFunc(title, body, base)
}

func (m *mockGitHubClient) ViewPR(identifier string) error {
	return nil
}

func (m *mockGitHubClient) PRStatus(identifier string) (cli.PRStatusResponse, error) {
	return cli.PRStatusResponse{}, nil
}

func (m *mockGitHubClient) MergePR(strategy cli.MergeStrategy) error {
	return nil
}

func TestRunPrCreate(t *testing.T) {
	tests := map[string]struct {
		args        []string
		wantExit    int
		wantExitErr string
		wantStdout  string
		prepare     func(t *testing.T, dir string)
		ghClient    cli.GitHubClienter
	}{
		"not a git repo": {
			args:        nil,
			wantExit:    1,
			wantExitErr: "Not a git repo",
		},
		"not authenticated": {
			args:        nil,
			wantExit:    1,
			wantExitErr: "Not authenticated with GitHub CLI, try running `gh auth login`",
			ghClient: &mockGitHubClient{
				authStatusFunc: func() error {
					return fmt.Errorf("Not authenticated with GitHub CLI, try running `gh auth login`")
				},
			},
			prepare: func(t *testing.T, dir string) {
				initRepo(t, dir)
			},
		},
		"with args": {
			args:        []string{"--title", "PR title", "--body", "PR body", "--base", "main"},
			wantExit:    0,
			wantExitErr: "",
			ghClient: &mockGitHubClient{
				authStatusFunc: func() error {
					return nil
				},
				createPRFunc: func(title, body, base string) error {
					return nil
				},
			},
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

			err := cli.Run(
				append(baseArgs, tc.args...),
				stdout,
				stderr,
				tc.ghClient,
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
