package cli

import (
	"fmt"
	"io"
	"log"
	"os/exec"
	"runtime"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/thomasgormley/dev-cli-go/internal/gh"
	"github.com/thomasgormley/dev-cli-go/internal/tui"
	"github.com/urfave/cli/v2"
)

func handlePRMerge(stdout, stderr io.Writer, ghCli gh.GitHubClienter) cli.ActionFunc {
	return func(c *cli.Context) error {
		if _, err := tea.LogToFile("/Users/thomas/dev/dev-cli-go/debug.log", "DEBUG"); err != nil {
			log.Fatal(err)
		}
		identifier := c.Args().First()
		p := tea.NewProgram(initialModel(identifier, ghCli), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return err
		}

		return nil
	}
}

var (
	docStyle  = lipgloss.NewStyle().Margin(1, 2)
	document  = lipgloss.NewStyle().Margin(1, 2).Align(lipgloss.Left)
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

type viewMode string

const (
	mergeSelectionView viewMode = "mergeSelection"
)

type handleMergeModel struct {
	// state
	suspended bool
	merged    bool
	content   string
	view      viewMode

	// PR
	title            string
	base             string
	head             string
	mergeStateStatus gh.MergeStateStatus

	// deps
	identifier string
	ghClient   gh.GitHubClienter

	// bubbles ui
	spinner       spinner.Model
	list          list.Model
	width, height int
	mergeButtons  tea.Model
}

func initialModel(identifier string, ghCli gh.GitHubClienter) handleMergeModel {
	s := spinner.New()
	s.Spinner = spinner.Ellipsis
	s.Style = lipgloss.NewStyle().Foreground(primaryColour)

	d := NewDelegate()

	l := list.New([]list.Item{}, d, 0, 0)
	l.Title = "Status Checks"

	l.AdditionalShortHelpKeys = func() []key.Binding {
		return keys.ShortHelp()
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return keys.ShortHelp()
	}

	return handleMergeModel{
		suspended:        true,
		mergeStateStatus: "",
		spinner:          tui.NewEllipsisSpinner(),
		list:             l,
		identifier:       identifier,
		ghClient:         ghCli,
		mergeButtons:     tui.NewMergeButtons(),
	}
}

func (m handleMergeModel) Init() tea.Cmd {
	log.Println("initing")
	return tea.Batch(
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

var codeHighlight = lipgloss.NewStyle().Foreground(primaryColour).Bold(true).Background(lipgloss.Color(primaryHighlightBg))

func (m handleMergeModel) View() string {
	if m.suspended {
		return lipgloss.JoinVertical(
			lipgloss.Center,
			document.Render("loading", m.spinner.View()),
		)
	}

	title := docStyle.Render(
		fmt.Sprintf("# %s\t", m.title),
		fmt.Sprintf("(%s -> %s)", codeHighlight.Render(m.head), codeHighlight.Render(m.base)),
	)

	var content string
	switch m.view {
	case mergeSelectionView:
		content = docStyle.Render(m.mergeButtons.View())
	default:
		content = docStyle.Render(m.content)
	}
	// if len(m.form.Errors()) > 0 {
	// 	return ""
	// }
	// spin := m.spinner.View() + " "
	// if m.form.State == huh.StateCompleted && !m.merged {
	// 	return docStyle.Render(spin + "Merging...")
	// } else if m.form.State == huh.StateAborted {
	// 	return docStyle.Render(spin + "Merging cancelled")
	// }
	// if m.mergeStateStatus != CLEAN && len(m.list.Items()) > 0 {
	// 	lipgloss.JoinVertical(
	// 		lipgloss.Left,
	// 		m.list.View(),
	// 	)
	// 	return m.list.View()
	// }
	// if m.mergeStateStatus == "" {
	// 	return docStyle.Render(spin + "Checking CI..." + " press q to quit")
	// }
	// if m.mergeStateStatus == CLEAN && !m.merged {
	// 	return docStyle.Render(m.form.View())
	// }
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		content,
	)
}

func (m handleMergeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// if msg.String() == "ctrl+c" {
		// 	return m, tea.Quit
		// }
		switch {
		case key.Matches(msg, keys.Enter):
			if item, ok := m.list.SelectedItem().(statusCheckItem); ok {
				openBrowser(item.url)
			}
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize((msg.Width-h)/2, msg.Height-v)
	case statusCheckCmd:
		m.mergeStateStatus = msg.mergeStateStatus
		m.base = msg.base
		m.head = msg.head
		m.title = msg.title
		m.suspended = false

		// return m, nil

		if len(msg.checks) == 0 && msg.mergeStateStatus == "" {
			return m, tea.Sequence(
				tea.Quit,
			)
		}

		switch msg.mergeStateStatus {
		case gh.CLEAN:
			m.view = mergeSelectionView
			return m, nil
		case gh.UNSTABLE:
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

	case mergeCmd:
		m.merged = true
		return m, tea.Sequence(
			tea.Println(docStyle.Render(fmt.Sprintf("%s Merged successfully", checkMark))),
			tea.Quit,
		)
	}

	switch m.view {
	case mergeSelectionView:
		mergeBtns, mergeCmd := m.mergeButtons.Update(msg)
		mergeModel, ok := mergeBtns.(tui.MergeButtons)
		if !ok {
			panic("error accessing mergeModel")
		}
		m.mergeButtons = mergeModel
		cmds = append(cmds, mergeCmd)
	default:
		m.list, cmd = m.list.Update(msg)
	}

	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
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
