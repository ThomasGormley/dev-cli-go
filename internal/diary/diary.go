package diary

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/thomasgormley/dev-cli-go/internal/git"
)

// Entry is a struct representation of a Diary Entry persisted to a File
type Entry struct {
	Date     time.Time
	Path     string
	Content  string
	Sections []ContentSection
}

// ContentSection represents a markdown section within an Entry
type ContentSection struct {
	Title string
	Lines []string
}

// FSRepository is the filesystem storage layer for Diary Entries
type FSRepository struct {
	basePath string
	entries  []*Entry // lazily-loaded
}

func NewFilesystemRepository() FSRepository {
	path, found := RepoPath()
	if !found {
		panic("Diary repo path not found")
	}

	return FSRepository{
		basePath: path,
	}
}

func (r FSRepository) NewEntry() error {
	today := time.Now()
	year, month, date := DateStringsFor(today)

	// Create directory path: docs/YYYY/MM/
	dirPath := path.Join(r.basePath, "docs", year, month)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file path: docs/YYYY/MM/YYYY-MM-DD.md
	filePath := path.Join(dirPath, date+".md")

	// Check if entry already exists
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("entry for %s already exists", date)
	}

	// Create basic template with carried over tasks
	dayName := today.Format("Monday")
	template := fmt.Sprintf("# %s %s\n\n", dayName, date)

	// Write template to file
	if err := os.WriteFile(filePath, []byte(template), 0644); err != nil {
		return fmt.Errorf("failed to write entry file: %w", err)
	}

	return nil
}

func (r FSRepository) Commit() error {
	docsDir := "docs"

	docsPath := path.Join(r.basePath, docsDir)
	if _, err := os.Stat(docsPath); os.IsNotExist(err) {
		return fmt.Errorf("directory %s does not exist", docsDir)
	}

	if err := os.Chdir(r.basePath); err != nil {
		return fmt.Errorf("failed to change to repository directory: %w", err)
	}
	if err := git.Add(docsDir); err != nil {
		return fmt.Errorf("failed to add changes to staging: %w", err)
	}

	hasChanges, err := git.HasUncommittedChanges(docsDir)
	if err != nil {
		return fmt.Errorf("failed to check for changes: %w", err)
	}

	if hasChanges {
		commitMessage, err := createCommitMessage(docsDir)
		if err != nil {
			return fmt.Errorf("failed to create commit message: %w", err)
		}

		if err := git.Commit(commitMessage); err != nil {
			return fmt.Errorf("failed to commit changes: %w", err)
		}
	}

	git.RebaseAbort()

	if err := git.PullRebase(); err != nil {
		return fmt.Errorf("failed to pull remote changes: %w", err)
	}

	if err := git.Push("origin", "main"); err != nil {
		return fmt.Errorf("failed to push to remote repository: %w", err)
	}

	return nil
}
