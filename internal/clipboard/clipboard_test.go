package clipboard

import (
	"runtime"
	"testing"
)

func TestFormatMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		url      string
		expected string
	}{
		{
			name:     "simple link",
			title:    "Test PR",
			url:      "https://github.com/user/repo/pull/123",
			expected: "[Test PR](https://github.com/user/repo/pull/123)",
		},
		{
			name:     "title with special characters",
			title:    "Fix: Update & Improve [Feature]",
			url:      "https://github.com/user/repo/pull/456",
			expected: "[Fix: Update & Improve [Feature]](https://github.com/user/repo/pull/456)",
		},
		{
			name:     "empty title",
			title:    "",
			url:      "https://github.com/user/repo/pull/789",
			expected: "[](https://github.com/user/repo/pull/789)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMarkdown(tt.title, tt.url)
			if result != tt.expected {
				t.Errorf("formatMarkdown() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatHTML(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		url      string
		expected string
	}{
		{
			name:     "simple link",
			title:    "Test PR",
			url:      "https://github.com/user/repo/pull/123",
			expected: "<a href=\"https://github.com/user/repo/pull/123\">Test PR</a>",
		},
		{
			name:     "title with ampersand (should be escaped)",
			title:    "Fix & Update",
			url:      "https://github.com/user/repo/pull/456",
			expected: "<a href=\"https://github.com/user/repo/pull/456\">Fix &amp; Update</a>",
		},
		{
			name:     "title with quotes (should be escaped)",
			title:    "Fix \"bug\" in code",
			url:      "https://github.com/user/repo/pull/789",
			expected: "<a href=\"https://github.com/user/repo/pull/789\">Fix &#34;bug&#34; in code</a>",
		},
		{
			name:     "title with less than and greater than (should be escaped)",
			title:    "Update <Component>",
			url:      "https://github.com/user/repo/pull/101",
			expected: "<a href=\"https://github.com/user/repo/pull/101\">Update &lt;Component&gt;</a>",
		},
		{
			name:     "url with special characters (should be escaped)",
			title:    "Test",
			url:      "https://example.com?param=<value>&other=\"test\"",
			expected: "<a href=\"https://example.com?param=&lt;value&gt;&amp;other=&#34;test&#34;\">Test</a>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatHTML(tt.title, tt.url)
			if result != tt.expected {
				t.Errorf("formatHTML() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestEscapeAppleScript(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special characters",
			input:    "simple text",
			expected: "simple text",
		},
		{
			name:     "backslash",
			input:    "path\\to\\file",
			expected: "path\\\\to\\\\file",
		},
		{
			name:     "double quotes",
			input:    "He said \"hello\"",
			expected: "He said \\\"hello\\\"",
		},
		{
			name:     "newline",
			input:    "line1\nline2",
			expected: "line1\\nline2",
		},
		{
			name:     "carriage return",
			input:    "line1\rline2",
			expected: "line1\\rline2",
		},
		{
			name:     "multiple special characters",
			input:    "path\\file \"name\"\nwith newline",
			expected: "path\\\\file \\\"name\\\"\\nwith newline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeAppleScript(tt.input)
			if result != tt.expected {
				t.Errorf("escapeAppleScript() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// Note: CopyLink() and copyMultiFormat() are not easily testable in unit tests
// as they require actual clipboard access via osascript. These should be tested manually.

func TestGet(t *testing.T) {
	content, err := Get()
	if err != nil {
		t.Skipf("Skipping test on %s: clipboard access not available", runtime.GOOS)
	}

	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		if content == "" {
			t.Log("Clipboard is empty, which is a valid state")
		} else {
			t.Logf("Successfully read clipboard content: %q", content)
		}
	} else {
		t.Skipf("Skipping test: unsupported OS %s", runtime.GOOS)
	}
}
