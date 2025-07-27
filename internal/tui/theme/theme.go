package theme

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/v2/compat"
)

// -------------------------------------------------------------------------
// Makeshift Theme Implementation
// -------------------------------------------------------------------------

// Theme interface for diff styling
type Theme interface {
	// Background colors
	BackgroundPanel() compat.AdaptiveColor
	DiffRemovedBg() compat.AdaptiveColor
	DiffAddedBg() compat.AdaptiveColor
	DiffContextBg() compat.AdaptiveColor
	DiffLineNumber() compat.AdaptiveColor
	DiffRemovedLineNumberBg() compat.AdaptiveColor
	DiffAddedLineNumberBg() compat.AdaptiveColor

	// Foreground colors
	Text() compat.AdaptiveColor
	TextMuted() compat.AdaptiveColor
	Error() compat.AdaptiveColor
	Success() compat.AdaptiveColor
	DiffRemoved() compat.AdaptiveColor
	DiffAdded() compat.AdaptiveColor
	DiffHighlightRemoved() compat.AdaptiveColor
	DiffHighlightAdded() compat.AdaptiveColor

	// Syntax highlighting colors
	SyntaxKeyword() compat.AdaptiveColor
	SyntaxType() compat.AdaptiveColor
	SyntaxFunction() compat.AdaptiveColor
	SyntaxVariable() compat.AdaptiveColor
	SyntaxString() compat.AdaptiveColor
	SyntaxNumber() compat.AdaptiveColor
	SyntaxComment() compat.AdaptiveColor
	SyntaxOperator() compat.AdaptiveColor
	SyntaxPunctuation() compat.AdaptiveColor
}

// defaultTheme provides a basic theme implementation
type defaultTheme struct{}

func (t defaultTheme) BackgroundPanel() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#ffffff"), Dark: lipgloss.Color("#1e1e1e")}
}
func (t defaultTheme) DiffRemovedBg() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#ffecec"), Dark: lipgloss.Color("#3f1515")}
}
func (t defaultTheme) DiffAddedBg() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#ecffec"), Dark: lipgloss.Color("#153f15")}
}
func (t defaultTheme) DiffContextBg() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#f8f8f8"), Dark: lipgloss.Color("#2d2d2d")}
}
func (t defaultTheme) DiffLineNumber() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#f0f0f0"), Dark: lipgloss.Color("#3a3a3a")}
}
func (t defaultTheme) DiffRemovedLineNumberBg() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#ffdddd"), Dark: lipgloss.Color("#4a1f1f")}
}
func (t defaultTheme) DiffAddedLineNumberBg() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#ddffdd"), Dark: lipgloss.Color("#1f4a1f")}
}

func (t defaultTheme) Text() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#000000"), Dark: lipgloss.Color("#ffffff")}
}
func (t defaultTheme) TextMuted() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#666666"), Dark: lipgloss.Color("#999999")}
}
func (t defaultTheme) Error() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#cc0000"), Dark: lipgloss.Color("#ff6666")}
}
func (t defaultTheme) Success() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#00cc00"), Dark: lipgloss.Color("#66ff66")}
}
func (t defaultTheme) DiffRemoved() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#cc0000"), Dark: lipgloss.Color("#ff6666")}
}
func (t defaultTheme) DiffAdded() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#00cc00"), Dark: lipgloss.Color("#66ff66")}
}
func (t defaultTheme) DiffHighlightRemoved() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#ff0000"), Dark: lipgloss.Color("#ff9999")}
}
func (t defaultTheme) DiffHighlightAdded() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#00ff00"), Dark: lipgloss.Color("#99ff99")}
}

func (t defaultTheme) SyntaxKeyword() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#0000ff"), Dark: lipgloss.Color("#6699ff")}
}
func (t defaultTheme) SyntaxType() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#008080"), Dark: lipgloss.Color("#66cccc")}
}
func (t defaultTheme) SyntaxFunction() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#800080"), Dark: lipgloss.Color("#cc99cc")}
}
func (t defaultTheme) SyntaxVariable() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#008000"), Dark: lipgloss.Color("#99cc99")}
}
func (t defaultTheme) SyntaxString() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#008000"), Dark: lipgloss.Color("#99cc99")}
}
func (t defaultTheme) SyntaxNumber() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#ff8000"), Dark: lipgloss.Color("#ffcc99")}
}
func (t defaultTheme) SyntaxComment() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#808080"), Dark: lipgloss.Color("#cccccc")}
}
func (t defaultTheme) SyntaxOperator() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#000000"), Dark: lipgloss.Color("#ffffff")}
}
func (t defaultTheme) SyntaxPunctuation() compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color("#000000"), Dark: lipgloss.Color("#ffffff")}
}

// CurrentTheme returns the current theme instance
func CurrentTheme() Theme {
	return defaultTheme{}
}
