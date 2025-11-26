package diary

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/thomasgormley/dev-cli-go/internal/git"
)

// Entry is a struct representation of a Diary Entry persisted to a File
type Entry struct {
	Date     time.Time
	Path     string
	Content  string
	Tasks    []TaskItem
	Sections []ContentSection
}

// TaskItem represents a TODO item within an Entry
type TaskItem struct {
	Text      string
	Completed bool
	Line      int
}

// TaskWithHistory extends TaskItem with historical information
type TaskWithHistory struct {
	TaskItem
	FirstSeen time.Time
	LastSeen  time.Time
	Locations []TaskLocation
}

// TaskLocation represents where a task appears in a file
type TaskLocation struct {
	TaskItem
	FilePath string
	Line     int
	Date     time.Time
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

func (r FSRepository) Commit(ctx context.Context) error {
	docsDir := "docs"

	// Check if docs directory exists
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

// extractIncompleteTasks finds and extracts incomplete tasks from previous diary entries
func (r FSRepository) extractIncompleteTasks(currentDate time.Time) ([]TaskItem, error) {
	extractor := newTaskExtractor()
	ctx := context.Background()

	docsDir := path.Join(r.basePath, "docs")
	var allTasks []TaskItem

	// Walk through all diary entry files in reverse chronological order
	err := filepath.Walk(docsDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-markdown files
		if info.IsDir() || !strings.HasSuffix(filePath, ".md") {
			return nil
		}

		// Extract date from filename and check if it's before current date
		fileName := filepath.Base(filePath)
		dateStr := strings.TrimSuffix(fileName, ".md")

		// Skip if filename doesn't match expected date format
		if len(dateStr) != 10 {
			return nil
		}

		entryDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return nil // Skip files that don't match date format
		}

		// Only process entries before the current date
		if !entryDate.Before(currentDate) {
			return nil
		}

		// Open and parse the file
		file, err := os.Open(filePath)
		if err != nil {
			return nil // Skip files that can't be opened
		}
		defer file.Close()

		tasks, err := extractor.extract(ctx, file)
		if err != nil {
			return nil // Skip files that can't be parsed
		}

		allTasks = append(allTasks, tasks...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk diary directory: %w", err)
	}

	// Deduplicate tasks
	deduplicatedTasks := extractor.deduplicateTasks(allTasks)

	return deduplicatedTasks, nil
}

// GetAllIncompleteTasks scans ALL diary entries and returns tasks with history
func (r FSRepository) GetAllIncompleteTasks() ([]TaskWithHistory, error) {
	extractor := newTaskExtractor()
	ctx := context.Background()

	docsDir := path.Join(r.basePath, "docs")
	taskMap := make(map[string]*TaskWithHistory) // Key: normalized task text

	// Walk through all diary entry files chronologically
	err := filepath.Walk(docsDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-markdown files
		if info.IsDir() || !strings.HasSuffix(filePath, ".md") {
			return nil
		}

		// Extract date from filename
		fileName := filepath.Base(filePath)
		dateStr := strings.TrimSuffix(fileName, ".md")

		// Skip if filename doesn't match expected date format
		if len(dateStr) != 10 {
			return nil
		}

		entryDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return nil // Skip files that don't match date format
		}

		// Open and parse the file
		file, err := os.Open(filePath)
		if err != nil {
			return nil // Skip files that can't be opened
		}
		defer file.Close()

		tasks, err := extractor.extract(ctx, file)
		if err != nil {
			return nil // Skip files that can't be parsed
		}

		// Process each task
		for _, task := range tasks {
			normalized := strings.ToLower(strings.TrimSpace(task.Text))

			if existing, found := taskMap[normalized]; found {
				// Update existing task
				existing.Locations = append(existing.Locations, TaskLocation{
					TaskItem: TaskItem{
						Text:      task.Text,
						Completed: false,
						Line:      task.Line,
					},
					FilePath: filePath,
					Line:     task.Line,
					Date:     entryDate,
				})
				if entryDate.Before(existing.FirstSeen) {
					existing.FirstSeen = entryDate
				}
				if entryDate.After(existing.LastSeen) {
					existing.LastSeen = entryDate
				}
			} else {
				// Create new task
				taskMap[normalized] = &TaskWithHistory{
					TaskItem: TaskItem{
						Text:      task.Text,
						Completed: false,
						Line:      task.Line,
					},
					FirstSeen: entryDate,
					LastSeen:  entryDate,
					Locations: []TaskLocation{{
						TaskItem: TaskItem{
							Text:      task.Text,
							Completed: false,
							Line:      task.Line,
						},
						FilePath: filePath,
						Line:     task.Line,
						Date:     entryDate,
					}},
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk diary directory: %w", err)
	}

	// Convert map to slice and sort by first seen date
	var result []TaskWithHistory
	for _, task := range taskMap {
		result = append(result, *task)
	}

	// Sort by first seen date (oldest first)
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].FirstSeen.After(result[j].FirstSeen) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result, nil
}

// MarkTaskComplete marks a specific task occurrence as complete using robust content matching
func (r FSRepository) MarkTaskComplete(location TaskLocation) error {
	// Read file
	file, err := os.Open(location.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read all lines
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Create regex pattern to match the specific task text
	// This handles various checkbox formats: - [ ], -[], -[ ], etc.
	taskPattern := regexp.MustCompile(`^(\s*)-\s*\[\s*\]\s*` + regexp.QuoteMeta(location.Text) + `\s*$`)

	// Search for the task in the file
	found := false
	for i, line := range lines {
		if taskPattern.MatchString(line) {
			// Replace with completed task
			indent := ""
			if matches := regexp.MustCompile(`^(\s*)`).FindStringSubmatch(line); len(matches) > 1 {
				indent = matches[1]
			}
			lines[i] = fmt.Sprintf("%s- [x] %s", indent, location.Text)
			found = true
			break
		}
	}

	if !found {
		// Fallback: try simple text-based search
		for i, line := range lines {
			if strings.Contains(line, location.Text) && strings.Contains(line, "- [") {
				// Replace incomplete task with completed task
				updatedLine := strings.Replace(line, "- [ ]", "- [x]", 1)
				updatedLine = strings.Replace(updatedLine, "-[]", "- [x]", 1)
				updatedLine = strings.Replace(updatedLine, "- [X]", "- [x]", 1)
				updatedLine = strings.Replace(updatedLine, "-[X]", "- [x]", 1)
				if updatedLine != line {
					lines[i] = updatedLine
					found = true
					break
				}
			}
		}
	}

	if !found {
		return fmt.Errorf("task '%s' not found in file %s", location.Text, location.FilePath)
	}

	// Write back to file
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(location.FilePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
