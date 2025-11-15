package print

import (
	"fmt"
	"io"
	"strings"
)

// ANSI color codes using opencode theme colors
const (
	SuccessColor = "7fd88f" // darkGreen from opencode theme
	ErrorColor   = "e06c75" // darkRed from opencode theme
	WarningColor = "f5a742" // darkOrange from opencode theme
	InfoColor    = "56b6c2" // darkCyan from opencode theme
	ResetColor   = "\x1b[0m"
)

// Common symbols for composition
const (
	Tick       = "✓"
	Cross      = "✗"
	WarningSym = "⚠"
	InfoSym    = "ℹ"
	Arrow      = "→"
	Bullet     = "•"
)

// colorize wraps text with ANSI color codes
func colorize(color, text string) string {
	return fmt.Sprintf("\x1b[38;2;%sm%s%s", color, text, ResetColor)
}

// Success prints a success message with variadic parts for composition
func Success(w io.Writer, parts ...string) {
	fmt.Fprintln(w, colorize(SuccessColor, strings.Join(parts, " ")))
}

// Error prints an error message with variadic parts for composition
func Error(w io.Writer, parts ...string) {
	fmt.Fprintln(w, colorize(ErrorColor, strings.Join(parts, " ")))
}

// Warning prints a warning message with variadic parts for composition
func Warning(w io.Writer, parts ...string) {
	fmt.Fprintln(w, colorize(WarningColor, strings.Join(parts, " ")))
}

// Info prints an info message with variadic parts for composition
func Info(w io.Writer, parts ...string) {
	fmt.Fprintln(w, colorize(InfoColor, strings.Join(parts, " ")))
}

// Successf prints a formatted success message in green
func Successf(w io.Writer, format string, args ...any) {
	fmt.Fprintln(w, colorize(SuccessColor, fmt.Sprintf(format, args...)))
}

// Errorf prints a formatted error message in red
func Errorf(w io.Writer, format string, args ...any) {
	fmt.Fprintln(w, colorize(ErrorColor, fmt.Sprintf(format, args...)))
}

// Warningf prints a formatted warning message in orange
func Warningf(w io.Writer, format string, args ...any) {
	fmt.Fprintln(w, colorize(WarningColor, fmt.Sprintf(format, args...)))
}

// Infof prints a formatted info message in cyan
func Infof(w io.Writer, format string, args ...any) {
	fmt.Fprintln(w, colorize(InfoColor, fmt.Sprintf(format, args...)))
}
