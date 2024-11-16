package tui

import (
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// type MergeModel struct {
// 	// mergeStateStatus cli.MergeStateStatus
// 	merged bool
// 	// strategy         cli.MergeStrategy
// 	messages   []string
// 	spinner    spinner.Model
// 	list       list.Model
// 	form       *huh.Form
// 	identifier string
// 	// ghClient         cli.GitHubClienter

// 	width, height int
// }

type MergeButtons struct {
	focused  int
	selected bool
	options  []string

	style lipgloss.Style
}

func NewMergeButtons() MergeButtons {
	return MergeButtons{
		focused:  0,
		selected: false,
		options: []string{
			"squash",
			"merge",
			"rebase",
		},
	}
}

func (m MergeButtons) Init() tea.Cmd {
	return nil
}

var (
	checkMark = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓")
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	dot       = lipgloss.NewStyle().Foreground(lipgloss.Color("236")).Render(" • ")
)

func (m MergeButtons) View() string {

	content := fmt.Sprintf("%s All checks have passed\n", checkMark)
	var btns string
	for i, b := range m.options {
		btns += fmt.Sprintf("%s\n", button(b, m.focused == i))
	}

	help := helpStyle.Render("j/k, up/down: select") + dot +
		helpStyle.Render("enter: choose") + dot +
		helpStyle.Render("q, esc: quit")
	return lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		lipgloss.JoinHorizontal(lipgloss.Left, btns),
		help,
	)
}

func button(label string, focused bool) string {
	if focused {
		return fmt.Sprintf("x [ %s ]", label)

	}
	return fmt.Sprintf("[ %s ]", label)
}

func (m MergeButtons) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.focused++
			if m.focused > 3 {
				m.focused = 3
			}
		case "k", "up":
			m.focused--
			if m.focused < 0 {
				m.focused = 0
			}
		case "enter", "space":
			m.selected = true
		}
	}

	log.Printf("updating focused %d,", m.focused)
	return m, nil
}
