package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

var (
	primaryColour      = lipgloss.Color("#F97415")
	primaryHighlightBg = lipgloss.Color("#451a03") // Darker shade to allow primary to pop on text
)

var (
	checkMark   = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓")
	SubtleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	dot         = lipgloss.NewStyle().Foreground(lipgloss.Color("236")).Render(" • ")
	btn         = lipgloss.NewStyle().Foreground(primaryColour)
	skipped     = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).SetString("■") // Light gray color
	failure     = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).SetString("✗") // Red color
	unknown     = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).SetString("?") // Red color
)

func NewEllipsisSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Ellipsis
	s.Style = lipgloss.NewStyle().Foreground(primaryColour)
	return s
}
