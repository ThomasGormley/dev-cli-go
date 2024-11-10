package cli

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/urfave/cli/v2"
)

func handlePRMerge(stdout, stderr io.Writer, ghCli GitHubClienter) cli.ActionFunc {
	return func(c *cli.Context) error {
		identifier := c.Args().First()
		p := tea.NewProgram(initialModel(identifier, ghCli))
		if _, err := p.Run(); err != nil {
			return err
		}

		return nil
	}
}

var (
	docStyle  = lipgloss.NewStyle().Margin(1, 2)
	checkMark = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓")
	skipped   = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).SetString("■") // Light gray color
	failure   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).SetString("✗") // Red color
)

var keys = keyMap{
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "open details"),
	),
}

type handleMergeModel struct {
	mergeStateStatus MergeStateStatus
	merged           bool
	strategy         MergeStrategy
	messages         []string
	spinner          spinner.Model
	list             list.Model
	form             *huh.Form
	identifier       string
	ghClient         GitHubClienter

	width, height int
}

func initialModel(identifier string, ghCli GitHubClienter) handleMergeModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	d := NewDelegate()

	l := list.New([]list.Item{}, d, 0, 0)
	l.Title = "Status Checks"

	l.AdditionalShortHelpKeys = func() []key.Binding {
		return keys.ShortHelp()
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return keys.ShortHelp()
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[MergeStrategy]().
				Key("mergeStrategy").
				Options(huh.NewOptions(MergeSquash, MergeCommit, MergeRebase)...).
				Title("Choose your merge strategy"),

			huh.NewConfirm().
				Key("confirm").
				Title("Confirm Merge").
				Validate(func(v bool) error {
					if !v {
						return fmt.Errorf("Welp, finish up then")
					}
					return nil
				}).
				Affirmative("Merge").
				Negative("Cancel"),
		).WithWidth(45).
			WithShowHelp(false).
			WithShowErrors(false),
	).WithTheme(huh.ThemeBase())

	return handleMergeModel{
		mergeStateStatus: "",
		messages:         []string{},
		spinner:          s,
		list:             l,
		form:             form,
		identifier:       identifier,
		ghClient:         ghCli,
	}
}

func (m handleMergeModel) Init() tea.Cmd {
	return tea.Batch(
		m.form.Init(),
		m.spinner.Tick,
		awaitStatusCheckCmd(m.identifier, m.ghClient),
	)
}

// Checking CI...
// Checks Complete :tick:
// Merge strategy?
// - [x] Squash & Merge
// - [] Merge Commit
// - [] Rebase
// Merging...
// Merged :tick:
// --exit--

func (m handleMergeModel) View() string {
	if len(m.form.Errors()) > 0 {
		return ""
	}
	spin := m.spinner.View() + " "
	if m.form.State == huh.StateCompleted && !m.merged {
		return docStyle.Render(spin + "Merging...")
	} else if m.form.State == huh.StateAborted {
		return docStyle.Render(spin + "Merging cancelled")
	}
	if m.mergeStateStatus == UNSTABLE && len(m.list.Items()) > 0 {
		lipgloss.JoinVertical(
			lipgloss.Left,
			m.list.View(),
		)
		return m.list.View()
	}
	if m.mergeStateStatus == "" {
		return docStyle.Render(spin + "Checking CI..." + " press q to quit")
	}
	if m.mergeStateStatus == CLEAN && !m.merged {
		return docStyle.Render(m.form.View())
	}
	return ""
}

func (m handleMergeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch {
		case key.Matches(msg, keys.Enter):
			if item, ok := m.list.SelectedItem().(statusCheckItem); ok {
				openBrowser(item.url)
			}
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd // Continuously tick the spinner
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize((msg.Width-h)/2, msg.Height-v)
	case statusCheckCmd:
		m.mergeStateStatus = msg.mergeStateStatus

		switch msg.mergeStateStatus {
		case CLEAN:
			return m, tea.Println(docStyle.Render(fmt.Sprintf("%s All checks have passed", checkMark)))
		case UNSTABLE:
			m.list.Title = "Some checks were unsuccessful, cannot merge"
			return m, tea.Sequence(
				tea.EnterAltScreen,
				m.list.SetItems(msg.checks),
			)
		default:
			m.list.Title = fmt.Sprintf("Unable to merge, unhandled status: %s", msg.mergeStateStatus)
			return m, tea.Sequence(
				tea.EnterAltScreen,
				m.list.SetItems(msg.checks),
			)

		}

	case mergeResponse:
		m.merged = true
		return m, tea.Sequence(
			tea.Println(docStyle.Render(fmt.Sprintf("%s Merged successfully", checkMark))),
			tea.Quit,
		)
	}

	var cmds []tea.Cmd

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
		cmds = append(cmds, cmd)
	}
	if m.form.State == huh.StateCompleted {
		s := m.form.GetString("mergeStrategy")
		cmds = append(cmds, awaitMerge(MergeStrategy(s)))
	} else if len(m.form.Errors()) > 0 {
		return m, tea.Quit

	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

type mergeResponse int

func awaitMerge(strategy MergeStrategy) tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		return mergeResponse(200)
	})
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
