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

	"github.com/AlecAivazis/survey/v2"
	"github.com/thomasgormley/dev-cli-go/internal/diary"
	"github.com/thomasgormley/dev-cli-go/internal/editor"
	"github.com/thomasgormley/dev-cli-go/internal/print"
	"github.com/thomasgormley/dev-cli-go/internal/spinner"
	"github.com/urfave/cli/v2"
)

var diaryDir = path.Join(os.Getenv("HOME"), "dev", "engineering-diary")

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

		// Always open the repository first
		cmd := prepareCmd(c.Context, os.Stdin, stdout, stderr, editorPath, append(editorArgs, diaryRepo)...)
		if err := cmd.Start(); err != nil {
			return cli.Exit(err, 1)
		}

		// Only open today's entry if not repo-only
		if !repoOnly {
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
			if err != nil {
				return cli.Exit(err, 1)
			}

			if lineCount <= 3 {
				entryPath = entryPath + fmt.Sprintf(":%d:1", lineCount)
			}

			// Open the file
			cmd = prepareCmd(c.Context, os.Stdin, stdout, stderr, editorPath, append(editorArgs, entryPath)...)
			if err := cmd.Start(); err != nil {
				return cli.Exit(err, 1)
			}
		}

		return nil
	}
}

func handleDiarySync(_, _ io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		repo := diary.NewFilesystemRepository()

		err := spinner.WithContext(c.Context, "Syncing Diary repository with remote...", repo.Commit,
			spinner.WithSuccessMessage("Synced to remote"),
			spinner.WithFailureMessage("Could not sync to remote"),
		)
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

func handleDiaryTasks(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		repo := diary.NewFilesystemRepository()

		// Get all incomplete tasks with history
		tasks, err := repo.GetAllIncompleteTasks()
		if err != nil {
			print.Error(stderr, "Failed to load tasks:", err.Error())
			return cli.Exit("", 1)
		}

		if len(tasks) == 0 {
			print.Success(stdout, print.Tick, "No incomplete tasks found!")
			return nil
		}

		// Show task count (following PR handler pattern)
		print.Info(stdout,
			"Found",
			print.ColorNote(fmt.Sprintf("%d", len(tasks))),
			"incomplete tasks:",
		)
		print.Info(stdout) // Empty line for spacing

		// Format for survey: "Task text (first seen: 2024-10-24)"
		var options []string
		for _, task := range tasks {
			displayText := task.Text + print.ColorNote(fmt.Sprintf(" %s (%s)", print.Bullet,
				task.FirstSeen.Format("2006-01-02")))
			options = append(options, displayText)
		}

		// Interactive selection using existing survey dependency
		var selected []int
		prompt := &survey.MultiSelect{
			Message: "Select tasks to mark as complete:",
			Options: options,
		}

		if err := survey.AskOne(prompt, &selected); err != nil {
			print.Error(stderr, "Selection cancelled:", err.Error())
			return cli.Exit("", 1)
		}

		if len(selected) == 0 {
			print.Info(stdout, "No tasks selected")
			return nil
		}

		// Mark selected tasks as complete (most recent occurrence)
		print.Info(stdout, "Marking tasks as complete...")
		successCount := 0

		for _, idx := range selected {
			task := tasks[idx]
			// Use most recent location (last in slice)
			location := task.Locations[len(task.Locations)-1]

			if err := repo.MarkTaskComplete(location); err != nil {
				print.Error(stderr,
					print.Cross,
					"Failed to mark task complete:",
					task.Text,
					"-",
					err.Error())
				continue
			}

			print.Success(stdout, print.Tick, "Marked complete:", task.Text)
			successCount++
		}

		// Summary (following PR handler pattern)
		if successCount > 0 {
			print.Info(stdout, print.Wrap(
				print.ColorNote("Summary:"),
				fmt.Sprintf("%d", successCount),
				"tasks marked as complete",
			))
		}

		return nil
	}
}
