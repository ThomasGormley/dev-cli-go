package cli

import "testing"

func TestPrTitleFromBranch(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ABC-123-some-description", "ABC-123: some description"},
		{"prefix-ABC-123-some-description", "ABC-123: some description"},
		{"ABC-123", ""},
		{"invalid-branch", ""},
		{"ABC-123-some", "ABC-123: some"},
		{"abc-123-some", "ABC-123: some"},
	}
	for _, test := range tests {
		result := prTitleFromBranch(test.input)
		if result != test.expected {
			t.Errorf("For input %q, expected %q, got %q", test.input, test.expected, result)
		}
	}
}
