package diary

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
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

	// Extract incomplete TODOs from previous entries
	incompleteTasks, err := r.extractIncompleteTasks(today)
	if err != nil {
		return fmt.Errorf("failed to extract incomplete tasks: %w", err)
	}

	// Create basic template with carried over tasks
	dayName := today.Format("Monday")
	template := fmt.Sprintf("# %s %s\n\n", dayName, date)

	// Add TODO section if there are incomplete tasks
	if len(incompleteTasks) > 0 {
		template += "## TODO\n\n"
		for _, task := range incompleteTasks {
			template += fmt.Sprintf("- [ ] %s\n", task.Text)
		}
		template += "\n"
	}

	// Write template to file
	if err := os.WriteFile(filePath, []byte(template), 0644); err != nil {
		return fmt.Errorf("failed to write entry file: %w", err)
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

// MarkTaskComplete marks a specific task occurrence as complete
func (r FSRepository) MarkTaskComplete(location TaskLocation) error {
	// Read the file
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

	// Update the specific line (convert to 0-based index)
	if location.Line < 1 || location.Line > len(lines) {
		return fmt.Errorf("line number %d out of range", location.Line)
	}

	lineIndex := location.Line - 1
	originalLine := lines[lineIndex]

	// Replace incomplete task with completed task
	updatedLine := strings.Replace(originalLine, "- [ ]", "- [x]", 1)
	if updatedLine == originalLine {
		return fmt.Errorf("task not found in incomplete state at line %d", location.Line)
	}

	lines[lineIndex] = updatedLine

	// Write back to file
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(location.FilePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
