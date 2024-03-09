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
		wantStdErr []byte
		wantStdOut []byte
		prepare    func(t *testing.T, dir string)
	}{
		"not a git repo": {
			args:       nil,
			wantStdErr: []byte("Not a git repository\n"),
			wantStdOut: []byte("Creating a new pull request\n"),
			prepare:    nil,
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
			err := cli.Run(append(baseArgs, tc.args...), nil, stdout, stderr)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tc.wantStdErr != nil {
				if !bytes.Equal(stderr.Bytes(), tc.wantStdErr) {
					t.Errorf("Expected stderr: %s, got: %s", string(tc.wantStdErr), stderr.String())
				}
			}

			if tc.wantStdOut != nil {
				if !bytes.Equal(stdout.Bytes(), tc.wantStdOut) {
					t.Errorf("Expected stdout: %s, got: %s", string(tc.wantStdOut), stdout.String())
				}
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
