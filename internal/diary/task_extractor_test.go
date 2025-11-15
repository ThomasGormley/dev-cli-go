package diary

import (
	"context"
	"strings"
	"testing"
)

func TestTaskExtractor_extract(t *testing.T) {
	extractor := newTaskExtractor()

	tests := []struct {
		name      string
		input     string
		wantTasks []TaskItem
		wantError bool
	}{
		{
			name: "simple TODO section with incomplete tasks",
			input: `# Monday 2024-12-15

## TODO
- [ ] Task one
- [x] Completed task
- [ ] Task two

## Other Section
Some content here`,
			wantTasks: []TaskItem{
				{Text: "Task one", Completed: false, Line: 4},
				{Text: "Task two", Completed: false, Line: 6},
			},
			wantError: false,
		},
		{
			name: "no TODO section",
			input: `# Monday 2024-12-15

## Work
- [ ] Some task

## Personal
- [ ] Another task`,
			wantTasks: []TaskItem{},
			wantError: false,
		},
		{
			name: "TODO section with only completed tasks",
			input: `# Monday 2024-12-15

## TODO
- [x] Completed task one
- [X] Completed task two

## Other Section
Some content`,
			wantTasks: []TaskItem{},
			wantError: false,
		},
		{
			name: "multiple TODO sections",
			input: `# Monday 2024-12-15

## TODO
- [ ] First task

## Work
Some work content

## TODO
- [ ] Second task
- [x] Completed task`,
			wantTasks: []TaskItem{
				{Text: "First task", Completed: false, Line: 4},
				{Text: "Second task", Completed: false, Line: 10},
			},
			wantError: false,
		},
		{
			name: "tasks with various formatting",
			input: `# Monday 2024-12-15

## TODO
- [ ] Task with [brackets]
- [ ] Task with - dashes
- [ ]   Task with extra spaces
- [ ] Task with trailing spaces   
- [] Task with empty brackets
- [x] Completed task`,
			wantTasks: []TaskItem{
				{Text: "Task with [brackets]", Completed: false, Line: 4},
				{Text: "Task with - dashes", Completed: false, Line: 5},
				{Text: "Task with extra spaces", Completed: false, Line: 6},
				{Text: "Task with trailing spaces", Completed: false, Line: 7},
				{Text: "Task with empty brackets", Completed: false, Line: 8},
			},
			wantError: false,
		},
		{
			name:      "empty input",
			input:     "",
			wantTasks: []TaskItem{},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			reader := strings.NewReader(tt.input)

			gotTasks, err := extractor.extract(ctx, reader)

			if (err != nil) != tt.wantError {
				t.Errorf("extract() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if len(gotTasks) != len(tt.wantTasks) {
				t.Errorf("extract() got %d tasks, want %d tasks", len(gotTasks), len(tt.wantTasks))
				return
			}

			for i, got := range gotTasks {
				want := tt.wantTasks[i]
				if got.Text != want.Text {
					t.Errorf("extract() task[%d].Text = %v, want %v", i, got.Text, want.Text)
				}
				if got.Completed != want.Completed {
					t.Errorf("extract() task[%d].Completed = %v, want %v", i, got.Completed, want.Completed)
				}
				if got.Line != want.Line {
					t.Errorf("extract() task[%d].Line = %v, want %v", i, got.Line, want.Line)
				}
			}
		})
	}
}

func TestTaskExtractor_deduplicateTasks(t *testing.T) {
	extractor := newTaskExtractor()

	tests := []struct {
		name     string
		tasks    []TaskItem
		expected []TaskItem
	}{
		{
			name: "no duplicates",
			tasks: []TaskItem{
				{Text: "Task one", Completed: false, Line: 1},
				{Text: "Task two", Completed: false, Line: 2},
			},
			expected: []TaskItem{
				{Text: "Task one", Completed: false, Line: 1},
				{Text: "Task two", Completed: false, Line: 2},
			},
		},
		{
			name: "exact duplicates",
			tasks: []TaskItem{
				{Text: "Task one", Completed: false, Line: 1},
				{Text: "Task one", Completed: false, Line: 3},
				{Text: "Task two", Completed: false, Line: 5},
			},
			expected: []TaskItem{
				{Text: "Task one", Completed: false, Line: 1},
				{Text: "Task two", Completed: false, Line: 5},
			},
		},
		{
			name: "case insensitive duplicates",
			tasks: []TaskItem{
				{Text: "Task one", Completed: false, Line: 1},
				{Text: "TASK ONE", Completed: false, Line: 3},
				{Text: "task one", Completed: false, Line: 5},
			},
			expected: []TaskItem{
				{Text: "Task one", Completed: false, Line: 1},
			},
		},
		{
			name: "whitespace variations",
			tasks: []TaskItem{
				{Text: "Task one", Completed: false, Line: 1},
				{Text: "  Task one  ", Completed: false, Line: 3},
				{Text: "\tTask one\t", Completed: false, Line: 5},
			},
			expected: []TaskItem{
				{Text: "Task one", Completed: false, Line: 1},
			},
		},
		{
			name: "mixed duplicates",
			tasks: []TaskItem{
				{Text: "Task one", Completed: false, Line: 1},
				{Text: "Task two", Completed: false, Line: 2},
				{Text: "task one", Completed: false, Line: 3},
				{Text: "Task three", Completed: false, Line: 4},
				{Text: "TASK TWO", Completed: false, Line: 5},
			},
			expected: []TaskItem{
				{Text: "Task one", Completed: false, Line: 1},
				{Text: "Task two", Completed: false, Line: 2},
				{Text: "Task three", Completed: false, Line: 4},
			},
		},
		{
			name:     "empty slice",
			tasks:    []TaskItem{},
			expected: []TaskItem{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.deduplicateTasks(tt.tasks)

			if len(result) != len(tt.expected) {
				t.Errorf("deduplicateTasks() got %d tasks, want %d tasks", len(result), len(tt.expected))
				return
			}

			for i, got := range result {
				want := tt.expected[i]
				if got.Text != want.Text {
					t.Errorf("deduplicateTasks() task[%d].Text = %v, want %v", i, got.Text, want.Text)
				}
				if got.Line != want.Line {
					t.Errorf("deduplicateTasks() task[%d].Line = %v, want %v", i, got.Line, want.Line)
				}
			}
		})
	}
}
