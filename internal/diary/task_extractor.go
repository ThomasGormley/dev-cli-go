package diary

import (
	"bufio"
	"context"
	"io"
	"regexp"
	"strings"
)

// taskExtractor extracts TaskItem objects from markdown content
type taskExtractor struct {
	// inTodoSection tracks whether we're currently in a TODO section
	inTodoSection bool
	// taskRegex matches markdown task items with optional indentation
	taskRegex *regexp.Regexp
	// sectionRegex matches TODO section headers
	sectionRegex *regexp.Regexp
}

// newTaskExtractor creates a new taskExtractor instance
func newTaskExtractor() *taskExtractor {
	return &taskExtractor{
		taskRegex:    regexp.MustCompile(`^(\s*)-\s*\[([ xX]?)\]\s*(.*)$`),
		sectionRegex: regexp.MustCompile(`^##\s*TODO\s*$`),
	}
}

// extract reads from io.Reader and extracts incomplete TaskItem objects
func (te *taskExtractor) extract(ctx context.Context, r io.Reader) ([]TaskItem, error) {
	var tasks []TaskItem
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		line := scanner.Text()
		lineNum++

		// Check if we're entering a TODO section
		if te.sectionRegex.MatchString(line) {
			te.inTodoSection = true
			continue
		}

		// Check if we've hit another section (starts with # but not TODO)
		if strings.HasPrefix(line, "#") && !te.sectionRegex.MatchString(line) {
			te.inTodoSection = false
			continue
		}

		// If we're in the TODO section, try to extract task items
		if te.inTodoSection {
			if matches := te.taskRegex.FindStringSubmatch(line); matches != nil {
				status := strings.ToLower(matches[2])
				text := strings.TrimSpace(matches[3])

				// Only extract incomplete tasks
				if status == " " || status == "" {
					task := TaskItem{
						Text:      text,
						Completed: false,
						Line:      lineNum,
					}
					tasks = append(tasks, task)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

// deduplicateTasks removes duplicate tasks based on text content
// Keeps the first occurrence (most recent) and removes subsequent duplicates
func (te *taskExtractor) deduplicateTasks(tasks []TaskItem) []TaskItem {
	seen := make(map[string]bool)
	var deduplicated []TaskItem

	for _, task := range tasks {
		// Normalize task text for comparison (trim and lowercase)
		normalized := strings.ToLower(strings.TrimSpace(task.Text))

		if !seen[normalized] {
			seen[normalized] = true
			deduplicated = append(deduplicated, task)
		}
	}

	return deduplicated
}
