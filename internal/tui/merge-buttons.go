package tui

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/thomasgormley/dev-cli-go/internal/gh"
)

// type MergeModel struct {
// 	// mergeStateStatus gh.gh.MergeStateStatus
// 	merged bool
// 	// strategy         gh.MergeStrategy
// 	messages   []string
// 	spinner    spinner.Model
// 	list       list.Model
// 	form       *huh.Form
// 	identifier string
// 	// ghClient         gh.GitHubClienter

// 	width, height int
// }

type MergeButtons struct {
	focused       int
	cancelling    bool
	merged        bool
	selected      bool
	options       []string
	ticksTilMerge int

	// ui
	spinner spinner.Model

	gh gh.GitHubClienter
}

func NewMergeButtons() MergeButtons {
	return MergeButtons{
		focused:  0,
		selected: false,
		merged:   false,
		options: []string{
			"squash",
			"merge",
			"rebase",
		},
		spinner:       NewEllipsisSpinner(),
		ticksTilMerge: 50,
	}
}

func (m MergeButtons) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m MergeButtons) ellipsis(strs ...string) string {
	return lipgloss.JoinHorizontal(lipgloss.Left, strings.Join(strs, " "), m.spinner.View())
}

func (m MergeButtons) strategy() gh.MergeStrategy {
	if !m.selected {
		return ""
	}
	switch m.options[m.focused] {
	case "squash":
		return gh.MergeSquash
	case "merge":
		return gh.MergeCommit
	case "rebase":
		return gh.MergeRebase
	default:
		log.Fatalf("unable to determine strategy from options/focused")
		return ""
	}
}

func (m MergeButtons) View() string {
	if m.cancelling {
		return m.ellipsis("cancelling merge")
	}
	if m.selected {
		return mergeView(m)
	}
	return selectView(m)
}

func selectView(m MergeButtons) string {
	statusMessage := fmt.Sprintf("%s All checks have passed\n\n", checkMark)
	maxLabelWidth := 0
	for _, b := range m.options {
		if len(b) > maxLabelWidth {
			maxLabelWidth = len(b)
		}
	}

	statusMessage += "choose a merge strategy"
	var btns string
	for i, b := range m.options {
		// Pad the label to the max width
		paddedLabel := fmt.Sprintf("%-*s", maxLabelWidth, b)
		btns += fmt.Sprintf("%s\n", button(paddedLabel, m.focused == i))
	}

	help := helpStyle.Render("j/k, up/down: select") + dot +
		helpStyle.Render("enter: choose") + dot +
		helpStyle.Render("q, esc: quit")
	return lipgloss.JoinVertical(
		lipgloss.Left,
		statusMessage,
		btns,
		help,
	)
}

func mergeView(m MergeButtons) string {
	if m.merged {
		return fmt.Sprintf("%s pull request merged", checkMark)
	}
	content := m.ellipsis("merging in", string(m.ticksTilMerge))
	if m.ticksTilMerge == 0 {
		content = m.ellipsis("merging")
	}
	return content
}

func (m MergeButtons) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd // Continuously tick the spinner
	}

	if m.selected {
		return updateMerge(msg, m)
	}
	return updateSelect(msg, m)
}

type tickMsg struct{}

func tick() tea.Cmd {
	log.Println("calling tick")
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		log.Println("ticking")
		return tickMsg{}
	})
}

type mergeCancelMsg struct{}

func cancelMerge() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		// simulate time spent for msg to appear
		return mergeCancelMsg{}
	})
}

func updateMerge(msg tea.Msg, m MergeButtons) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.cancelling = true
			return m, cancelMerge()
		}
	case tickMsg:
		if m.ticksTilMerge == 0 {
			return m, merge(m.strategy(), m.gh)
		}
		m.ticksTilMerge--
		return m, tick()
	case mergedMsg:
		m.merged = true
	case mergeCancelMsg:
		// reset
		m = NewMergeButtons()
	}

	return m, nil

}

func updateSelect(msg tea.Msg, m MergeButtons) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
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
			return m, tick()
		}
	}

	return m, nil
}

func button(label string, focused bool) string {
	if focused {
		return btn.Render(fmt.Sprintf("[ %s ]", label))

	}
	return btn.Render("[ ") + label + btn.Render(" ]")
}

type mergedMsg struct{}

func merge(strategy gh.MergeStrategy, ghCli gh.GitHubClienter) tea.Cmd {
	return tea.Tick(time.Second*2, func(time.Time) tea.Msg {
		log.Printf("fake merging with strategy %s\n", strategy)
		return mergedMsg{}
	})
	// return func() tea.Msg {
	// err := ghgh.MergePR(strategy)
	// if err != nil {
	// 	// return err command
	// 	return nil
	// }
	// return mergedMsg{}
	// }
}
