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
	checkMark = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓")
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	dot       = lipgloss.NewStyle().Foreground(lipgloss.Color("236")).Render(" • ")
	btn       = lipgloss.NewStyle().Foreground(primaryColour)
)

func NewEllipsisSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Ellipsis
	s.Style = lipgloss.NewStyle().Foreground(primaryColour)
	return s
}
