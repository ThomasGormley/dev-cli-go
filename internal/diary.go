package cli

import (
	"io"
	"path"
	"time"

	"github.com/urfave/cli/v2"
)

const diaryDir = "/Users/thomas/dev/engineering-diary"

func handleDiaryNew(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		today := time.Now()
		stdout.Write([]byte("Creating a new diary entry for " + today.Format("2006-01-02") + "...\n"))
		newEntryScript := path.Join(diaryDir, "scripts", "new-entry.sh")
		err := prepareCmd(nil, stdout, stderr, newEntryScript).Run()
		if err != nil {
			return cli.Exit(err, 1)
		}

		return nil
	}
}

func handleDiaryOpen(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		today := time.Now()
		stdout.Write([]byte("Opening today's diary entry, " + today.Format("2006-01-02") + "...\n"))
		openEntryScript := path.Join(diaryDir, "scripts", "open.sh")
		err := prepareCmd(nil, stdout, stderr, openEntryScript).Run()
		if err != nil {
			return cli.Exit(err, 1)
		}

		return nil
	}
}

func handleDiarySync(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		stdout.Write([]byte("Syncing diary entries...\n"))
		syncScript := path.Join(diaryDir, "scripts", "commit-changes.sh")
		err := prepareCmd(nil, stdout, stderr, syncScript).Run()
		if err != nil {
			return cli.Exit(err, 1)
		}

		return nil
	}
}
