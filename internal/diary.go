package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/thomasgormley/dev-cli-go/internal/diary"
	"github.com/thomasgormley/dev-cli-go/internal/editor"
	"github.com/urfave/cli/v2"
)

var diaryDir = path.Join(os.Getenv("HOME"), "dev", "engineering-diary")

func handleDiaryNew(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		if err := diary.NewEntry(); err != nil {
			return cli.Exit(err, 1)
		}

		return nil
	}
}

func handleDiaryOpen(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		today := time.Now()
		// stdout.Write([]byte("Opening today's diary entry, " + today.Format(time.DateOnly) + "...\n"))
		// err := prepareCmd(nil, stdout, stderr, path.Join(diaryDir, "scripts", "open.sh")).Run()
		// if err != nil {
		// 	return cli.Exit(err, 1)
		// }

		editorPath, editorArgs, ok := editor.Lookup()
		if !ok {
			return cli.Exit("$EDITOR not set, can't open diary entry", 1)
		}

		diaryRepo, ok := diary.RepoPath()
		if !ok {
			return cli.Exit("Diary repo path not found", 1)
		}

		entryPath, err := diary.EnsureEntryExists(today)
		if err != nil {
			return cli.Exit(err, 1)
		}

		// Check how many lines the entryPath file is
		file, err := os.Open(entryPath)
		if err != nil {
			return cli.Exit(err, 1)
		}
		defer file.Close()

		lineCount, err := lineCounter(file)

		if lineCount <= 3 {
			entryPath = entryPath + fmt.Sprintf(":%d:1", lineCount)
		}

		// So we don't open the file in some random repo window we need to
		// open the repository first...
		cmd := prepareCmd(c.Context, os.Stdin, stdout, stderr, editorPath, append(editorArgs, diaryRepo)...)

		if err := cmd.Start(); err != nil {
			return cli.Exit(err, 1)
		}

		// then the file...
		cmd = prepareCmd(c.Context, os.Stdin, stdout, stderr, editorPath, append(editorArgs, entryPath)...)

		if err := cmd.Start(); err != nil {
			return cli.Exit(err, 1)
		}

		return nil
	}
}

func handleDiarySync(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		stdout.Write([]byte("Syncing diary entries...\n"))
		err := prepareCmd(c.Context, nil, stdout, stderr, path.Join(diaryDir, "scripts", "commit-changes.sh")).Run()
		if err != nil {
			return cli.Exit(err, 1)
		}

		return nil
	}
}

func prepareCmd(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = stdout
	cmd.Stdin = stdin
	cmd.Stderr = stderr

	return cmd
}

func lineCounter(r io.Reader) (int, error) {
	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := r.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count, nil

		case err != nil:
			return count, err
		}
	}
}
