package print

import (
	"bytes"
	"strings"
	"testing"
)

func TestColorize(t *testing.T) {
	text := "test message"
	colored := colorize(SuccessColor, text)
	expected := "\x1b[38;2;7fd88fmtest message\x1b[0m"

	if colored != expected {
		t.Errorf("Expected %q, got %q", expected, colored)
	}
}

func TestSuccess(t *testing.T) {
	var buf bytes.Buffer
	Success(&buf, "test success")

	output := buf.String()
	expected := "\x1b[38;2;7fd88fmtest success\x1b[0m\n"

	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestSuccessVariadic(t *testing.T) {
	var buf bytes.Buffer
	Success(&buf, Tick, "test success")

	output := buf.String()
	expected := "\x1b[38;2;7fd88fm✓ test success\x1b[0m\n"

	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestError(t *testing.T) {
	var buf bytes.Buffer
	Error(&buf, "test error")

	output := buf.String()
	expected := "\x1b[38;2;e06c75mtest error\x1b[0m\n"

	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestErrorVariadic(t *testing.T) {
	var buf bytes.Buffer
	Error(&buf, Cross, "test error")

	output := buf.String()
	expected := "\x1b[38;2;e06c75m✗ test error\x1b[0m\n"

	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestWarning(t *testing.T) {
	var buf bytes.Buffer
	Warning(&buf, "test warning")

	output := buf.String()
	expected := "\x1b[38;2;f5a742mtest warning\x1b[0m\n"

	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestWarningVariadic(t *testing.T) {
	var buf bytes.Buffer
	Warning(&buf, WarningSym, "test warning")

	output := buf.String()
	expected := "\x1b[38;2;f5a742m⚠ test warning\x1b[0m\n"

	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestInfo(t *testing.T) {
	var buf bytes.Buffer
	Info(&buf, "test info")

	output := buf.String()
	expected := "\x1b[38;2;56b6c2mtest info\x1b[0m\n"

	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestInfoVariadic(t *testing.T) {
	var buf bytes.Buffer
	Info(&buf, InfoSym, "test info")

	output := buf.String()
	expected := "\x1b[38;2;56b6c2mℹ test info\x1b[0m\n"

	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestSuccessf(t *testing.T) {
	var buf bytes.Buffer
	Successf(&buf, "test %s", "success")

	output := buf.String()
	expected := "\x1b[38;2;7fd88fmtest success\x1b[0m\n"

	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestErrorf(t *testing.T) {
	var buf bytes.Buffer
	Errorf(&buf, "test %s", "error")

	output := buf.String()
	expected := "\x1b[38;2;e06c75mtest error\x1b[0m\n"

	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestWarningf(t *testing.T) {
	var buf bytes.Buffer
	Warningf(&buf, "test %s", "warning")

	output := buf.String()
	expected := "\x1b[38;2;f5a742mtest warning\x1b[0m\n"

	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestInfof(t *testing.T) {
	var buf bytes.Buffer
	Infof(&buf, "test %s", "info")

	output := buf.String()
	expected := "\x1b[38;2;56b6c2mtest info\x1b[0m\n"

	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestMultipleMessages(t *testing.T) {
	var buf bytes.Buffer

	Success(&buf, "first")
	Error(&buf, "second")
	Warning(&buf, "third")
	Info(&buf, "fourth")

	output := buf.String()
	lines := strings.Split(output, "\n")

	// Should have 4 lines + 1 empty line from final newline
	if len(lines) != 5 {
		t.Errorf("Expected 5 lines, got %d", len(lines))
	}

	// Check each line has correct color
	if !strings.Contains(lines[0], SuccessColor) {
		t.Error("First line should have success color")
	}
	if !strings.Contains(lines[1], ErrorColor) {
		t.Error("Second line should have error color")
	}
	if !strings.Contains(lines[2], WarningColor) {
		t.Error("Third line should have warning color")
	}
	if !strings.Contains(lines[3], InfoColor) {
		t.Error("Fourth line should have info color")
	}
}
