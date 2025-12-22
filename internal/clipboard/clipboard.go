package clipboard

import (
	"fmt"
	"html"
	"os/exec"
	"strings"
)

// CopyLink copies a link to the clipboard in both markdown and HTML formats.
// Applications will automatically use the format they prefer:
// - Slack, Notion, rich text editors: Use HTML (renders as clickable link)
// - Plain text editors: Use markdown fallback [title](url)
func CopyLink(title, url string) error {
	markdown := formatMarkdown(title, url)
	htmlLink := formatHTML(title, url)

	return copyMultiFormat(markdown, htmlLink)
}

// formatMarkdown creates a markdown-formatted link
func formatMarkdown(title, url string) string {
	return fmt.Sprintf("[%s](%s)", title, url)
}

// formatHTML creates an HTML anchor tag with proper escaping
func formatHTML(title, url string) string {
	return fmt.Sprintf("<a href=\"%s\">%s</a>",
		html.EscapeString(url),
		html.EscapeString(title))
}

// copyMultiFormat copies both markdown and HTML to macOS clipboard using osascript
func copyMultiFormat(plainText, htmlText string) error {
	// Escape strings for AppleScript
	plainText = escapeAppleScript(plainText)
	htmlText = escapeAppleScript(htmlText)

	script := fmt.Sprintf(`
		set markdownText to "%s"
		set htmlText to "%s"
		set the clipboard to {text:markdownText, «class HTML»:htmlText}
	`, plainText, htmlText)

	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}

	return nil
}

// escapeAppleScript escapes special characters for AppleScript strings
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	return s
}
