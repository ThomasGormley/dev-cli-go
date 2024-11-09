package cli

import (
	"fmt"
	"io"

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

var docStyle = lipgloss.NewStyle().Margin(1, 2)

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
	return handleMergeModel{
		spinner:    s,
		list:       list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
		identifier: identifier,
		ghClient:   ghCli,
	}
}

func (m handleMergeModel) Init() tea.Cmd {
	return checkStatus(m.identifier, m.ghClient)
}

func (m handleMergeModel) View() string {
	if len(m.list.Items()) > 0 {
		return docStyle.Render(m.list.View())
	}
	str := fmt.Sprintf("\n\n   %s Loading forever...press q to quit\n\n", m.spinner.View())
	return str
}

func (m handleMergeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
	case PRStatusResponse:
		checkItems := make([]list.Item, 0)

		for _, check := range msg.CurrentBranch.StatusCheckRollup {
			checkItems = append(checkItems, statusCheckItemFrom(check))
		}

		lister := list.New(checkItems, list.NewDefaultDelegate(), 0, 0)
		h, v := docStyle.GetFrameSize()
		lister.SetSize(m.list.Width()-h, m.list.Height()-v)
		lister.Title = "Status Checks"
		m.list = lister
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func statusCheckItemFrom(rollup StatusCheckRollup) statusCheckItem {
	name := rollup.Name
	if name == "" {
		name = rollup.Context
	}
	conclusion := rollup.Conclusion
	if conclusion == "" {
		conclusion = rollup.State
	}
	return statusCheckItem{
		name:       name,
		conclusion: conclusion,
		url:        rollup.DetailsURL,
	}
}

type statusCheckItem struct {
	name       string
	conclusion string
	url        string
}

func (i statusCheckItem) Title() string       { return i.name }
func (i statusCheckItem) Description() string { return i.conclusion }
func (i statusCheckItem) FilterValue() string { return i.name }
