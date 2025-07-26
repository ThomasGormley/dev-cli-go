package tui

import (
	"context"
	"fmt"
	"log"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/go-github/v74/github"
	"github.com/thomasgormley/dev-cli-go/internal/tui/components"
	"github.com/thomasgormley/dev-cli-go/internal/tui/components/diff"
)

var _ tea.Model = &Model{}

func NewModel(ghClient *github.Client) tea.Model {
	vp := viewport.New(50, 20)
	vp.SetContent("Select a comment to view details...")
	return &Model{
		github:       ghClient,
		diffViewport: vp,
		commentsList: components.NewCommentsList(),
	}
}

type Model struct {
	width, height int
	github        *github.Client

	diffViewport viewport.Model
	commentsList components.CommentsList
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(fetchPRComments(context.TODO(), m.github, 2))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "ctrl+b":
			m.diffViewport.ViewUp()
		case "ctrl+f":
			m.diffViewport.ViewDown()
		case "up":
			m.diffViewport.LineUp(1)
		case "down":
			m.diffViewport.LineDown(1)
		}
	case tea.WindowSizeMsg:
		msg.Height -= 2
		m.width, m.height = msg.Width, msg.Height
		m.diffViewport.Width = m.width - lipgloss.Width(m.commentsList.View())
		m.diffViewport.Height = m.height - lipgloss.Height(m.renderHeader())
	case components.CommentsSelectedMsg:
		m = m.updateDiffViewport(msg.Comment)
	case error:
		log.Printf("error: %v", msg)
	}

	log.Printf("viewport offset: %d", m.diffViewport.YOffset)
	updatedCommentsList, cmd := m.commentsList.Update(msg)
	m.commentsList = updatedCommentsList.(components.CommentsList)
	cmds = append(cmds, cmd)

	// updatedViewport, cmd := m.diffViewport.Update(msg)
	// m.diffViewport = updatedViewport
	// cmds = append(cmds, cmd)

	return m, cmd
}

func (m Model) updateDiffViewport(comment *github.PullRequestComment) Model {
	if comment != nil {
		diffHunk := comment.GetDiffHunk()
		// starts := comment.GetStartLine()
		// ends := comment.GetLine()
		if diffHunk == "" {
			diffHunk = "No diff context available for this comment"
			// starts = 0
		}
		diff, _ := diff.FormatDiff(
			comment.GetPath(),
			diffHunk,
			diff.WithWidth(m.width-2-lipgloss.Width(m.commentsList.View())),
		)
		m.diffViewport.SetContent(diff)
		// m.diffViewport.SetYOffset(starts)
	} else {
		m.diffViewport.SetYOffset(0)
		m.diffViewport.SetContent("Select a comment to view diff context...")
	}

	return m
}

var (
	docStyle = lipgloss.NewStyle().Margin(1, 2)
)

func (m Model) View() string {
	// Add header with comment count and help text
	header := m.renderHeader()

	// Calculate available height for content
	contentHeight := m.height - lipgloss.Height(header) - 12

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		lipgloss.PlaceVertical(
			contentHeight,
			lipgloss.Left,
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				docStyle.Render(m.commentsList.View()),
				docStyle.Render(m.diffViewport.View()),
			),
		),
	)
}

func fetchPRComments(ctx context.Context, client *github.Client, id int) tea.Cmd {
	return func() tea.Msg {
		log.Println("Fetching PR comments")
		comments, _, err := client.PullRequests.ListComments(ctx, "thomasgormley", "dev-cli-go", id, nil)
		if err != nil {
			return nil
		}
		// b, err := json.MarshalIndent(comments, "", "  ")
		// if err != nil {
		// 	log.Println("error marshaling comments to JSON:", err)
		// } else {
		// 	log.Println(string(b))
		// }
		return components.CommentsUpdatedMsg{Comments: comments}
	}
}

// renderHeader creates a header with comment count and help information
func (m Model) renderHeader() string {
	commentCount, topLevelCount, replyCount := m.commentsList.GetStats()

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00d7ff")).
		Render("PR Comments Review")

	stats := fmt.Sprintf("Total: %d comments (%d top-level, %d replies)",
		commentCount, topLevelCount, replyCount)

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#808080")).
		Render("Navigate: ↑↓/j/k • Quit: q/Ctrl+C")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		stats,
		help,
		"",
	)
}
