package cli

import (
	"strings"
	"testing"
)

func TestListTests(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []TestInfo
	}{
		{
			name:  "single test function",
			input: `internal/pr_test.go:5:func TestPrTitleFromBranch(t *testing.T) {`,
			expected: []TestInfo{
				{Name: "TestPrTitleFromBranch", PackagePath: "./internal/...", FileName: "internal/pr_test.go"},
			},
		},
		{
			name: "multiple test functions in one file",
			input: `internal/example_test.go:10:func TestOne(t *testing.T) {
internal/example_test.go:20:func TestTwo(t *testing.T) {
internal/example_test.go:30:func TestThree(t *testing.T) {`,
			expected: []TestInfo{
				{Name: "TestOne", PackagePath: "./internal/...", FileName: "internal/example_test.go"},
				{Name: "TestTwo", PackagePath: "./internal/...", FileName: "internal/example_test.go"},
				{Name: "TestThree", PackagePath: "./internal/...", FileName: "internal/example_test.go"},
			},
		},
		{
			name: "multiple files with tests",
			input: `internal/pr_test.go:5:func TestPrTitleFromBranch(t *testing.T) {
internal/another_test.go:5:func TestExampleOne(t *testing.T) {
internal/another_test.go:14:func TestExampleTwo(t *testing.T) {`,
			expected: []TestInfo{
				{Name: "TestPrTitleFromBranch", PackagePath: "./internal/...", FileName: "internal/pr_test.go"},
				{Name: "TestExampleOne", PackagePath: "./internal/...", FileName: "internal/another_test.go"},
				{Name: "TestExampleTwo", PackagePath: "./internal/...", FileName: "internal/another_test.go"},
			},
		},
		{
			name:  "test with underscores and numbers",
			input: `internal/complex_test.go:1:func TestWithUnderscores_AndNumbers123(t *testing.T) {`,
			expected: []TestInfo{
				{Name: "TestWithUnderscores_AndNumbers123", PackagePath: "./internal/...", FileName: "internal/complex_test.go"},
			},
		},
		{
			name:     "empty input",
			input:    "",
			expected: []TestInfo{},
		},
		{
			name: "non-test functions should be ignored",
			input: `internal/helper.go:10:func someHelper() {
internal/test.go:20:func TestValidFunction(t *testing.T) {
internal/main.go:5:func main() {`,
			expected: []TestInfo{
				{Name: "TestValidFunction", PackagePath: "./internal/...", FileName: "internal/test.go"},
			},
		},
		{
			name: "TestMain should be filtered out",
			input: `internal/example_test.go:5:func TestMain(m *testing.M) {
internal/example_test.go:10:func TestValid(t *testing.T) {`,
			expected: []TestInfo{
				{Name: "TestValid", PackagePath: "./internal/...", FileName: "internal/example_test.go"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			result, err := ListTests(reader)

			if err != nil {
				t.Fatalf("ListTests() returned error: %v", err)
			}

			// Check if slices are equal
			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %d tests, got %d", len(tt.expected), len(result))
			}

			for i, expected := range tt.expected {
				if i >= len(result) {
					t.Errorf("Missing test at index %d: %+v", i, expected)
					continue
				}

				actual := result[i]
				if actual.Name != expected.Name {
					t.Errorf("Test %d: expected name %s, got %s", i, expected.Name, actual.Name)
				}
				if actual.PackagePath != expected.PackagePath {
					t.Errorf("Test %d: expected package path %s, got %s", i, expected.PackagePath, actual.PackagePath)
				}
				if actual.FileName != expected.FileName {
					t.Errorf("Test %d: expected filename %s, got %s", i, expected.FileName, actual.FileName)
				}
			}
		})
	}
}

func TestListTestsMalformedInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "line with too few colons",
			input: "invalid-line-format",
		},
		{
			name:  "line with no function declaration",
			input: "file.go:10:some random text",
		},
		{
			name: "mixed valid and invalid lines",
			input: `invalid-line
internal/test.go:5:func TestValid(t *testing.T) {
another-invalid-line`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			result, err := ListTests(reader)

			if err != nil {
				t.Fatalf("ListTests() should not return error for malformed input: %v", err)
			}

			// For the mixed case, we should still get the valid test
			if tt.name == "mixed valid and invalid lines" {
				expected := []TestInfo{
					{Name: "TestValid", PackagePath: "./internal/...", FileName: "internal/test.go"},
				}

				if len(result) != len(expected) {
					t.Errorf("Expected %d tests, got %d", len(expected), len(result))
				} else if len(result) > 0 {
					if result[0].Name != expected[0].Name {
						t.Errorf("Expected test name %s, got %s", expected[0].Name, result[0].Name)
					}
				}
			}
		})
	}
}
