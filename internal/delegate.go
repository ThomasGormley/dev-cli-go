package cli

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

type keyMap struct {
	Enter key.Binding
}

func NewKeyMap() keyMap {
	return keyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open details"),
		),
	}
}

var (
	primaryColour      = lipgloss.Color("#F97415")
	primaryHighlightBg = lipgloss.Color("#451a03") // Darker shade to allow primary to pop on text
)

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter}
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Enter, k.Enter}, // first column
		{k.Enter},          // second column
	}
}

func NewDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles.NormalTitle = d.Styles.NormalTitle.Bold(true)
	d.Styles.SelectedTitle = lipgloss.NewStyle().
		Bold(true).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(primaryColour).
		Foreground(primaryColour).
		Padding(0, 0, 0, 1)

	d.Styles.SelectedDesc = d.Styles.SelectedTitle.
		Foreground(primaryColour)

	return d
}
