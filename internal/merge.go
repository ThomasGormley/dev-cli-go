package cli

import (
	"fmt"
	"io"
	"log"
	"os/exec"
	"runtime"

	"github.com/charmbracelet/bubbles/key"
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
	statusCheckView    viewMode = "statusChecks"
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
	spinner          spinner.Model
	width, height    int
	mergeModel       tea.Model
	statusCheckModel tea.Model
}

func initialModel(identifier string, ghCli gh.GitHubClienter) handleMergeModel {

	return handleMergeModel{
		// suspended:        true,
		view:             statusCheckView,
		spinner:          tui.NewEllipsisSpinner(),
		identifier:       identifier,
		ghClient:         ghCli,
		mergeModel:       tui.NewMergeButtons(ghCli),
		statusCheckModel: tui.NewPullRequestStatus(identifier, ghCli),
	}
}

func (m handleMergeModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.statusCheckModel.Init(),
		// awaitStatusCheckCmd(m.identifier, m.ghClient),
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
	// if m.suspended {
	// 	return lipgloss.JoinVertical(
	// 		lipgloss.Center,
	// 		document.Render("loading", m.spinner.View()),
	// 	)
	// }

	title := docStyle.Render(
		fmt.Sprintf("# %s\t", m.title),
		fmt.Sprintf("(%s -> %s)", codeHighlight.Render(m.head), codeHighlight.Render(m.base)),
	)

	var content string
	switch m.view {
	case mergeSelectionView:
		content = m.mergeModel.View()
	case statusCheckView:
		content = m.statusCheckModel.View()
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		docStyle.Render(title),
		docStyle.Render(content),
	)
}

func (m handleMergeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	switch m.view {
	case mergeSelectionView:
		mergeBtns, mergeCmd := m.mergeModel.Update(msg)
		mergeModel, ok := mergeBtns.(tui.MergeButtons)
		if !ok {
			panic("error accessing mergeModel")
		}
		m.mergeModel = mergeModel
		cmds = append(cmds, mergeCmd)
	case statusCheckView:
		updatedModel, updatedCmd := m.statusCheckModel.Update(msg)
		statusCheckModel, ok := updatedModel.(tui.PullRequestStatus)
		if !ok {
			panic("error accessing statusCheckModel")
		}
		m.statusCheckModel = statusCheckModel
		cmds = append(cmds, updatedCmd)
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
