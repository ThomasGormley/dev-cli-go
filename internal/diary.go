package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/thomasgormley/dev-cli-go/internal/clipboard"
	"github.com/thomasgormley/dev-cli-go/internal/diary"
	"github.com/thomasgormley/dev-cli-go/internal/editor"
	"github.com/thomasgormley/dev-cli-go/internal/spinner"
	"github.com/urfave/cli/v2"
)

func handleDiaryNew(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		repo := diary.NewFilesystemRepository()
		if err := repo.NewEntry(); err != nil {
			return cli.Exit(err, 1)
		}

		return nil
	}
}

func handleDiaryOpen(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		repoOnly := c.Bool("repo-only")
		today := time.Now()

		editorPath, editorArgs, ok := editor.Lookup()
		if !ok {
			editorPath = "zed"
		}

		diaryRepo, ok := diary.RepoPath()
		if !ok {
			return cli.Exit("Diary repo path not found", 1)
		}

		cmd := prepareCmd(c.Context, os.Stdin, stdout, stderr, editorPath, append(editorArgs, diaryRepo)...)
		if err := cmd.Start(); err != nil {
			return cli.Exit(err, 1)
		}

		if !repoOnly {
			entryPath, err := diary.EnsureEntryExists(today)
			if err != nil {
				return cli.Exit(err, 1)
			}

			file, err := os.Open(entryPath)
			if err != nil {
				return cli.Exit(err, 1)
			}
			defer file.Close()

			lineCount, err := lineCounter(file)
			if err != nil {
				return cli.Exit(err, 1)
			}

			if lineCount <= 3 {
				entryPath = entryPath + fmt.Sprintf(":%d:1", lineCount)
			}

			cmd = prepareCmd(c.Context, os.Stdin, stdout, stderr, editorPath, append(editorArgs, entryPath)...)
			if err := cmd.Start(); err != nil {
				return cli.Exit(err, 1)
			}
		}

		sync(c.Context, stderr)

		return nil
	}
}

func sync(ctx context.Context, stderr io.Writer) {
	repo := diary.NewFilesystemRepository()
	if err := repo.Commit(); err != nil {
		fmt.Fprintln(stderr, "Background sync error:", err)
	}
}

func handleDiarySync(_, _ io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		repo := diary.NewFilesystemRepository()

		err := spinner.WithContext(c.Context, "Syncing Diary repository with remote...", func(_ context.Context) error {
			return repo.Commit()
		},
			spinner.WithSuccessMessage("Synced to remote"),
			spinner.WithFailureMessage("Could not sync to remote"),
		)
		if err != nil {
			return cli.Exit(err, 1)
		}

		return nil
	}
}

func handleDiaryPaste(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		clipboardContent, err := clipboard.Get()
		if err != nil {
			return cli.Exit(err, 1)
		}

		if clipboardContent == "" {
			fmt.Fprintln(stdout, "Clipboard is empty. Nothing to append.")
			return nil
		}

		today := time.Now()
		entryExisted := diary.EntryExists(today)

		entryPath, err := diary.EnsureEntryExists(today)
		if err != nil {
			return cli.Exit(err, 1)
		}

		timestamp := today.Format("2006-01-02 15:04:05")
		formattedContent := fmt.Sprintf("%s\n```\n%s\n```\n", timestamp, clipboardContent)

		f, err := os.OpenFile(entryPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			return cli.Exit(fmt.Errorf("failed to open entry file: %w", err), 1)
		}
		defer f.Close()

		if _, err := f.WriteString(formattedContent); err != nil {
			return cli.Exit(fmt.Errorf("failed to append to entry: %w", err), 1)
		}

		if !entryExisted {
			fmt.Fprintf(stdout, "Created new diary entry and appended clipboard contents to: %s\n", entryPath)
		} else {
			fmt.Fprintf(stdout, "Appended clipboard contents to existing diary entry: %s\n", entryPath)
		}

		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "--- Appended content ---")
		fmt.Fprintln(stdout, clipboardContent)

		sync(c.Context, stderr)
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
