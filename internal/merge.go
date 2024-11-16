package cli

import (
	"fmt"
	"io"
	"log"
	"math"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
	"github.com/thomasgormley/dev-cli-go/internal/gh"
	"github.com/thomasgormley/dev-cli-go/internal/tui"
	"github.com/urfave/cli/v2"
)

func handlePRMerge(stdout, stderr io.Writer, ghCli gh.GitHubClienter) cli.ActionFunc {
	return func(c *cli.Context) error {
		if os.Args[0] == "devd" {
			if _, err := tea.LogToFile("/Users/thomas/dev/dev-cli-go/debug.log", "DEBUG"); err != nil {
				log.Fatal(err)
			}
		}
		log.Println(os.Args)
		identifier := c.Args().First()
		p := tea.NewProgram(initialModel(identifier, ghCli), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return err
		}

		return nil
	}
}

var (
	docStyle = lipgloss.NewStyle().Margin(1, 2)
)

type viewMode string

const (
	mergeSelectionView viewMode = "mergeSelection"
	statusCheckView    viewMode = "statusChecks"
)

type handleMergeModel struct {
	// state
	loaded bool
	err    error
	view   viewMode

	// PR
	prTitle string
	base    string
	head    string
	isDraft bool

	// deps
	identifier string
	ghClient   gh.GitHubClienter

	// bubbles ui
	spinner          spinner.Model
	width, height    int
	mergeModel       tea.Model
	statusCheckModel tui.PullRequestStatus
}

func initialModel(identifier string, ghCli gh.GitHubClienter) handleMergeModel {

	return handleMergeModel{
		loaded:           false,
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
		tui.CheckStatus(m.identifier, m.ghClient),
		m.statusCheckModel.Init(),
		m.mergeModel.Init(),
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
	if m.err != nil {
		return docStyle.Render(m.err.Error())
	}
	if !m.loaded {
		return lipgloss.JoinVertical(
			lipgloss.Center,
			docStyle.Render("loading", m.spinner.View()),
		)
	}

	var content string
	switch m.view {
	case mergeSelectionView:
		content = m.mergeModel.View()
	case statusCheckView:
		content = m.statusCheckModel.View()
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		docStyle.Render(m.title()),
		docStyle.Render(content),
	)
}

const maxWidth = 120

func (m handleMergeModel) title() string {
	min := int(math.Min(float64(m.width)-float64(docStyle.GetAlignHorizontal()), float64(maxWidth)))
	title := fmt.Sprintf("# %s\t", m.prTitle) + fmt.Sprintf("(%s -> %s)", codeHighlight.Render(m.head), codeHighlight.Render(m.base))
	if m.isDraft {
		title += tui.SubtleStyle.Bold(true).Render("\n\nDRAFT")
	}
	return wordwrap.String(
		title,
		min,
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
	case tui.CmdError:
		m.err = msg
		return m, tea.Sequence(tea.ExitAltScreen, tea.Quit)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		hMargin, vMargin := docStyle.GetFrameSize()
		titleHeight := lipgloss.Height(m.title())
		// Calculate available height for content
		availableHeight := msg.Height - titleHeight - vMargin - lipgloss.Height(m.statusCheckModel.View())
		if availableHeight < 0 {
			availableHeight = 0 // Prevent negative height
		}

		availableWidth := msg.Width - hMargin
		if availableWidth < 0 {
			availableWidth = 0 // Prevent negative widtg
		}
		m.statusCheckModel.SetListSize(availableWidth, availableHeight)
	case tui.StatusCheckMsg:
		m.prTitle = msg.Title
		m.base = msg.Base
		m.head = msg.Head
		m.loaded = true
		m.isDraft = msg.IsDraft
		if msg.MergeStateStatus == gh.CLEAN {
			m.view = mergeSelectionView
		}
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
