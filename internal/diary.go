package cli

import (
	"io"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/urfave/cli/v2"
)

var diaryDir = path.Join(os.Getenv("HOME"), "dev", "engineering-diary")

func handleDiaryNew(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		today := time.Now()
		stdout.Write([]byte("Creating a new diary entry for " + today.Format(time.DateOnly) + "...\n"))
		err := prepareCmd(nil, stdout, stderr, path.Join(diaryDir, "scripts", "new-entry.sh")).Run()
		if err != nil {
			return cli.Exit(err, 1)
		}

		return nil
	}
}

func handleDiaryOpen(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		today := time.Now()
		stdout.Write([]byte("Opening today's diary entry, " + today.Format(time.DateOnly) + "...\n"))
		err := prepareCmd(nil, stdout, stderr, path.Join(diaryDir, "scripts", "open.sh")).Run()
		if err != nil {
			return cli.Exit(err, 1)
		}

		return nil
	}
}

func handleDiarySync(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		stdout.Write([]byte("Syncing diary entries...\n"))
		err := prepareCmd(nil, stdout, stderr, path.Join(diaryDir, "scripts", "commit-changes.sh")).Run()
		if err != nil {
			return cli.Exit(err, 1)
		}

		return nil
	}
}

func prepareCmd(stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Stdout = stdout
	cmd.Stdin = stdin
	cmd.Stderr = stderr

	return cmd
}
