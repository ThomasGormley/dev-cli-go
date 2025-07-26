package tui

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/go-github/v74/github"
)

var _ tea.Model = &Model{}

func NewModel(ghClient *github.Client) tea.Model {
	vp := viewport.New(50, 20)
	vp.SetContent("Select a comment to view details...")
	return &Model{
		comments:     []*github.PullRequestComment{},
		github:       ghClient,
		diffViewport: vp,
	}
}

type Model struct {
	width, height int
	cursor        int
	comments      []*github.PullRequestComment
	github        *github.Client

	diffViewport viewport.Model
	flatComments []flatComment
}

type flatComment struct {
	comment *github.PullRequestComment
	depth   int
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(fetchPRComments(context.TODO(), m.github, 2))
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.updateDiffViewport()
			}
		case "down", "j":
			if m.cursor < len(m.flatComments)-1 {
				m.cursor++
				m.updateDiffViewport()
			}
		}
	case tea.WindowSizeMsg:
		msg.Height -= 2
		m.width, m.height = msg.Width, msg.Height
		m.diffViewport.Width = m.width / 2
		m.diffViewport.Height = m.height
	case prCommentsMsg:
		m.comments = msg.comments
		m.buildCommentTree()
		m.updateDiffViewport()
	case error:
		log.Printf("error: %v", msg)
	}
	return m, nil
}

func (m *Model) buildCommentTree() {
	// Create a map for quick lookup of comments by ID
	commentMap := make(map[int64]*github.PullRequestComment)
	for _, comment := range m.comments {
		commentMap[comment.GetID()] = comment
	}

	// Create a map to store children for each comment
	children := make(map[int64][]*github.PullRequestComment)
	var topLevel []*github.PullRequestComment

	// Group comments by parent
	for _, comment := range m.comments {
		if parentID := comment.GetInReplyTo(); parentID > 0 {
			if _, exists := commentMap[parentID]; exists {
				children[parentID] = append(children[parentID], comment)
			} else {
				// Parent doesn't exist, treat as top-level
				topLevel = append(topLevel, comment)
			}
		} else {
			topLevel = append(topLevel, comment)
		}
	}

	// Sort all comment groups by creation time
	sort.Slice(topLevel, func(i, j int) bool {
		return topLevel[i].GetCreatedAt().Before(topLevel[j].GetCreatedAt().Time)
	})
	for parentID := range children {
		sort.Slice(children[parentID], func(i, j int) bool {
			return children[parentID][i].GetCreatedAt().Before(children[parentID][j].GetCreatedAt().Time)
		})
	}

	// Flatten the tree into a navigable list
	m.flatComments = nil
	var addToFlat func(comment *github.PullRequestComment, depth int)
	addToFlat = func(comment *github.PullRequestComment, depth int) {
		m.flatComments = append(m.flatComments, flatComment{
			comment: comment,
			depth:   depth,
		})
		// Add children recursively
		for _, child := range children[comment.GetID()] {
			addToFlat(child, depth+1)
		}
	}

	// Add all top-level comments and their replies
	for _, comment := range topLevel {
		addToFlat(comment, 0)
	}
}

func (m *Model) updateDiffViewport() {
	if len(m.flatComments) > 0 && m.cursor < len(m.flatComments) {
		comment := m.flatComments[m.cursor].comment
		diffHunk := comment.GetDiffHunk()
		if diffHunk == "" {
			diffHunk = "No diff context available for this comment"
		}
		m.diffViewport.SetContent(diffHunk)
	} else {
		m.diffViewport.SetContent("Select a comment to view diff context...")
	}
}

var (
	docStyle = lipgloss.NewStyle().Margin(1, 2)
	indent   = "  "

	// Comment styling
	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#404040")).
			Foreground(lipgloss.Color("#ffffff"))

	usernameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00d7ff")).
			Bold(true)

	commentBodyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#d0d0d0"))

	timestampStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#808080"))

	dotStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))
)

func (m *Model) View() string {
	var commentsList string

	for i, flatComment := range m.flatComments {
		cursor := " "
		if m.cursor == i {
			cursor = "▶"
		}

		comment := flatComment.comment
		username := comment.GetUser().GetLogin()
		timestamp := comment.GetCreatedAt().Format("Jan 2, 15:04")
		body := strings.TrimSpace(comment.GetBody())

		if body == "" {
			body = "(empty comment)"
		}

		// Format header line: [Username] • Timestamp
		styledUsername := usernameStyle.Render(fmt.Sprintf("[%s]", username))
		dot := dotStyle.Render(" • ")
		styledTimestamp := timestampStyle.Render(timestamp)
		headerLine := fmt.Sprintf("%s %s%s%s",
			cursor,
			styledUsername,
			dot,
			styledTimestamp)

		// Format comment body with additional indentation
		styledBody := commentBodyStyle.Render(body)
		bodyLine := styledBody

		// Apply selection styling if this is the current item
		if m.cursor == i {
			headerLine = selectedStyle.Render(headerLine)
			bodyLine = selectedStyle.Render(bodyLine)
		}

		commentsList += headerLine + "\n" + bodyLine + "\n\n"

		// Add spacing between comment threads (top-level comments)
		if flatComment.depth == 0 && i < len(m.flatComments)-1 {
			// nextComment := m.flatComments[i+1]
			// if nextComment.depth == 0 {
			commentsList += "\n"
			// }
		}
	}

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
				docStyle.Render(commentsList),
				docStyle.Render(m.diffViewport.View()),
			),
		),
	)
}

type prCommentsMsg struct {
	comments []*github.PullRequestComment
}

func fetchPRComments(ctx context.Context, client *github.Client, id int) tea.Cmd {
	return func() tea.Msg {
		log.Println("Fetching PR comments")
		comments, _, err := client.PullRequests.ListComments(ctx, "thomasgormley", "dev-cli-go", id, nil)
		if err != nil {
			return nil
		}
		return prCommentsMsg{comments}
	}
}

// renderHeader creates a header with comment count and help information
func (m *Model) renderHeader() string {
	commentCount := len(m.flatComments)
	topLevelCount := 0
	replyCount := 0

	for _, comment := range m.flatComments {
		if comment.depth == 0 {
			topLevelCount++
		} else {
			replyCount++
		}
	}

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
