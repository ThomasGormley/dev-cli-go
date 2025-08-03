package diary

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/thomasgormley/dev-cli-go/internal/git"
)

func DateStringsFor(t time.Time) (year, month, full string) {
	year = t.Format("2006")
	month = t.Format("01")
	full = t.Format("2006-01-02")
	return
}

func RepoPath() (string, bool) {
	// return os.LookupEnv("DIARY_REPO")
	var diaryDir = path.Join(os.Getenv("HOME"), "dev", "engineering-diary")
	return diaryDir, true
}

func EntryPathFor(t time.Time) (string, error) {
	year, month, full := DateStringsFor(t)

	repo, ok := RepoPath()
	if !ok {
		return "", fmt.Errorf("DIARY_REPO environment variable not set")
	}

	return path.Join(repo, "docs", year, month, fmt.Sprintf("%s.md", full)), nil
}

func EntryExists(t time.Time) bool {
	path, err := EntryPathFor(t)

	if err != nil {
		return false
	}

	_, err = os.Stat(path)
	return err == nil
}

func EnsureEntryExists(t time.Time) (string, error) {
	entryPath, err := EntryPathFor(t)
	if err != nil {
		return "", err
	}

	// Check if the entry file exists
	_, statErr := os.Stat(entryPath)
	if statErr == nil {
		return entryPath, nil
	}

	// If not exists, create parent directories and the file
	dir := path.Dir(entryPath)
	if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
		return "", mkErr
	}

	if err := NewEntry(); err != nil {
		return "", err
	}

	return entryPath, nil
}

func NewEntry() error {
	repo, ok := RepoPath()
	if !ok {
		return errors.New("unable to find diary repo")
	}
	return exec.Command(path.Join(repo, "scripts", "new-entry.sh")).Run()

}

func SyncToRemote() error {
	repo, ok := RepoPath()
	if !ok {
		return errors.New("unable to find diary repo")
	}

	docsDir := "docs"

	// Check if docs directory exists
	docsPath := path.Join(repo, docsDir)
	if _, err := os.Stat(docsPath); os.IsNotExist(err) {
		return fmt.Errorf("directory %s does not exist", docsDir)
	}

	// Change to repository directory and check if it's a git repo
	if err := os.Chdir(repo); err != nil {
		return fmt.Errorf("failed to change to repository directory: %w", err)
	}

	// Check if it's a git repository
	if !git.IsRepo() {
		return errors.New("not a git repository")
	}

	// Add changes to staging area
	if err := git.Add(docsDir); err != nil {
		return fmt.Errorf("failed to add changes to staging: %w", err)
	}

	// Check for uncommitted changes
	hasChanges, err := git.HasUncommittedChanges(docsDir)
	if err != nil {
		return fmt.Errorf("failed to check for changes: %w", err)
	}

	if !hasChanges {
		return nil // No changes to commit
	}

	// Get file status and create commit message
	commitMessage, err := createCommitMessage(docsDir)
	if err != nil {
		return fmt.Errorf("failed to create commit message: %w", err)
	}

	// Commit the changes
	if err := git.Commit(commitMessage); err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	// Push to remote
	if err := git.Push("origin", "main"); err != nil {
		return fmt.Errorf("failed to push to remote repository: %w", err)
	}

	return nil
}

func createCommitMessage(dir string) (string, error) {
	statusOutput, err := git.Status(dir)
	if err != nil {
		return "", err
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	header := fmt.Sprintf("Changes committed on %s", timestamp)

	var addedFiles, modifiedFiles, deletedFiles, untrackedFiles []string

	lines := strings.Split(strings.TrimSpace(statusOutput), "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}

		status := line[:2]
		file := line[3:]

		switch status {
		case " M", "M ":
			modifiedFiles = append(modifiedFiles, "    "+file)
		case " A", "A ":
			addedFiles = append(addedFiles, "    "+file)
		case " D", "D ":
			deletedFiles = append(deletedFiles, "    "+file)
		case "??":
			untrackedFiles = append(untrackedFiles, "    "+file)
		}
	}

	var messageParts []string
	messageParts = append(messageParts, header)

	if len(addedFiles) > 0 {
		messageParts = append(messageParts, "Added:\n"+strings.Join(addedFiles, "\n"))
	}
	if len(modifiedFiles) > 0 {
		messageParts = append(messageParts, "Modified:\n"+strings.Join(modifiedFiles, "\n"))
	}
	if len(deletedFiles) > 0 {
		messageParts = append(messageParts, "Deleted:\n"+strings.Join(deletedFiles, "\n"))
	}
	if len(untrackedFiles) > 0 {
		messageParts = append(messageParts, "Untracked:\n"+strings.Join(untrackedFiles, "\n"))
	}

	return strings.Join(messageParts, "\n\n"), nil
}
