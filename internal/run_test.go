package cli_test

import (
	"bytes"
	"os"
	"testing"

	cli "github.com/thomasgormley/dev-cli-go/internal"
	urfave "github.com/urfave/cli/v2"
)

func TestRunPrCreate(t *testing.T) {
	tests := map[string]struct {
		args        []string
		wantExit    int
		wantExitErr string
		wantStdOut  string
		prepare     func(t *testing.T, dir string)
	}{
		"not a git repo": {
			args:        nil,
			wantExit:    1,
			wantExitErr: "Not a git repo",
		},
	}

	baseArgs := []string{"dev", "pr", "create"}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			dir := t.TempDir()
			if tc.prepare != nil {
				tc.prepare(t, dir)
			}
			os.Chdir(dir)

			err := cli.Run(append(baseArgs, tc.args...), nil, stdout, stderr, func(c *urfave.Context, err error) {})

			if exitErr, ok := err.(urfave.ExitCoder); ok {
				// check exit code is not nil and matches expected
				if got := exitErr.ExitCode(); got != tc.wantExit {
					t.Errorf("exit code: got %d, want %d", got, tc.wantExit)
				}

				// check error message exits and matches expected
				if got := err.Error(); got != tc.wantExitErr {
					t.Errorf("exit error: got %q, want %q", got, tc.wantExitErr)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if got := stdout.String(); tc.wantStdOut != "" && got != tc.wantStdOut {
				t.Errorf("stdout: got %q, want %q", got, tc.wantStdOut)
			}
		})
	}
}

// func initRepo(t *testing.T, dir string) {
// 	errBuf := &bytes.Buffer{}
// 	inBuf := &bytes.Buffer{}
// 	outBuf := &bytes.Buffer{}
// 	client := Client{
// 		RepoDir: dir,
// 		Stderr:  errBuf,
// 		Stdin:   inBuf,
// 		Stdout:  outBuf,
// 	}
// 	cmd, err := client.Command(context.Background(), []string{"init", "--quiet"}...)
// 	assert.NoError(t, err)
// 	_, err = cmd.Output()
// 	assert.NoError(t, err)
// }
