package cli

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/urfave/cli/v2"
)

func handlePRMerge(stdout, stderr io.Writer, ghCli GitHubClienter) cli.ActionFunc {
	return func(c *cli.Context) error {
		identifier := c.Args().First()
		p := tea.NewProgram(initialModel(identifier, ghCli), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return err
		}

		return nil
	}
}

func checkStatus(identifier string, ghCli GitHubClienter) func() tea.Msg {
	return func() tea.Msg {
		status, err := ghCli.PRStatus(identifier)
		if err != nil {
			return nil
		}

		return status
	}
}

var (
	docStyle  = lipgloss.NewStyle().Margin(1, 2)
	checkMark = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓")
	skipped   = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).SetString("■") // Light gray color
	failure   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).SetString("✗") // Red color
)

type keyMap struct {
	Enter key.Binding
}

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

var keys = keyMap{
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "open details"),
	),
}

type handleMergeModel struct {
	spinner    spinner.Model
	list       list.Model
	identifier string
	ghClient   GitHubClienter
}

func initialModel(identifier string, ghCli GitHubClienter) handleMergeModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	d := list.NewDefaultDelegate()
	d.Styles.NormalTitle = d.Styles.NormalTitle.Bold(true)
	d.Styles.SelectedTitle = lipgloss.NewStyle().
		Bold(true).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("#F97415")).
		Foreground(lipgloss.Color("#F97415")).
		Padding(0, 0, 0, 1)

	d.Styles.SelectedDesc = d.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#F97415"))

	l := list.New([]list.Item{}, d, 0, 0)
	l.Title = "Status Checks"

	l.AdditionalShortHelpKeys = func() []key.Binding {
		return keys.ShortHelp()
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return keys.ShortHelp()
	}
	return handleMergeModel{
		spinner:    s,
		list:       l,
		identifier: identifier,
		ghClient:   ghCli,
	}
}

func (m handleMergeModel) Init() tea.Cmd {
	return checkStatus(m.identifier, m.ghClient)
}

func (m handleMergeModel) View() string {
	if len(m.list.Items()) > 0 {
		return m.list.View()
	}
	str := fmt.Sprintf("\n\n   %s Checking CI... press q to quit\n\n", m.spinner.View())
	return str
}

func (m handleMergeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch {
		case key.Matches(msg, keys.Enter):
			item := m.list.SelectedItem().(statusCheckItem)
			if err := openBrowser(item.url); err != nil {

			}
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	case PRStatusResponse:
		checkItems := make([]list.Item, 0)
		for _, check := range msg.CurrentBranch.StatusCheckRollup {
			checkItems = append(checkItems, statusCheckItem{
				name:       check.Name,
				context:    check.Context,
				conclusion: check.Conclusion,
				state:      check.State,
				url:        check.DetailsURL,
			})
		}

		m.list.SetItems(checkItems)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func openBrowser(url string) error {
	if url == "" {
		return nil
	}
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}

type statusCheckItem struct {
	name       string
	context    string
	conclusion string
	state      string
	url        string
}

func (i statusCheckItem) Title() string {
	if i.name == "" {
		return i.context
	}
	return i.name
}

func (i statusCheckItem) Description() string {
	desc := i.conclusion
	if i.conclusion == "" {
		desc = i.state
	}
	return withIcon(desc)
}

func (i statusCheckItem) FilterValue() string {
	if i.name == "" {
		return i.context
	}
	return i.name
}

func withIcon(s string) string {
	var icon string
	switch strings.ToLower(s) {
	case "success":
		icon = checkMark.String()
	case "skipped":
		icon = skipped.String()
	case "failure":
		icon = failure.String()
	}
	return fmt.Sprintf("%s %s", icon, s)
}
