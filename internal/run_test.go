package cli_test

import (
	"bytes"
	"os"
	"testing"

	cli "github.com/thomasgormley/dev-cli-go/internal"
)

func TestRunPrCreate(t *testing.T) {
	tests := map[string]struct {
		args       []string
		wantStdErr string
		wantStdOut string
		prepare    func(t *testing.T, dir string)
	}{
		"not a git repo": {
			args:       nil,
			wantStdErr: "Not a git repo\n",
			wantStdOut: "Creating a new pull request\n",
			prepare: func(t *testing.T, dir string) {
				// do nothing
			},
		},
	}

	baseArgs := []string{"dev", "pr", "create"}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			dir := t.TempDir()
			tc.prepare(t, dir)
			os.Chdir(dir)
			err := cli.Run(append(baseArgs, tc.args...), nil, stdout, stderr)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if got := stderr.String(); got != tc.wantStdErr {
				t.Errorf("stderr: got %q, want %q", got, tc.wantStdErr)
			}

			if got := stdout.String(); got != tc.wantStdOut {
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
