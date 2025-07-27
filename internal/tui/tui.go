package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/go-github/v74/github"
	"github.com/thomasgormley/dev-cli-go/internal/tui/components"
	"github.com/thomasgormley/dev-cli-go/internal/tui/components/diff"
)

var _ tea.Model = &Model{}

func NewModel(ghClient *github.Client) tea.Model {
	vp := viewport.New(0, 0)
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
	commentsList components.CommentsView
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
			// case "up":
			// 	m.diffViewport.LineUp(1)
			// case "down":
			// 	m.diffViewport.LineDown(1)
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

		// Calculate header height
		headerHeight := lipgloss.Height(m.renderHeader())

		// Available height for content after header
		contentHeight := m.height - headerHeight

		// Split remaining height between components
		m.commentsList.Width = m.width
		m.commentsList.Height = contentHeight / 2
		m.diffViewport.Width = m.width
		m.diffViewport.Height = contentHeight / 2
	case components.CommentsSelectedMsg:
		m = m.updateDiffViewport(msg.Comment)
	case error:
		log.Printf("error: %v", msg)
	}

	updatedCommentsList, cmd := m.commentsList.Update(msg)
	m.commentsList = updatedCommentsList.(components.CommentsView)
	cmds = append(cmds, cmd)

	// updatedViewport, cmd := m.diffViewport.Update(msg)
	// m.diffViewport = updatedViewport
	// cmds = append(cmds, cmd)

	return m, cmd
}

func (m Model) updateDiffViewport(comment *github.PullRequestComment) Model {
	m.diffViewport.SetContent("")

	if comment != nil {
		diffHunk := comment.GetDiffHunk()
		var starts int
		isMultiline := comment.StartLine != nil
		if isMultiline {
			starts = comment.GetOriginalStartLine()
		} else {
			starts = comment.GetOriginalLine()
		}
		// ends := comment.GetLine()
		if diffHunk == "" {
			diffHunk = "No diff context available for this comment"
			starts = 0
		}
		diff, _ := diff.FormatDiff(
			comment.GetPath(),
			diffHunk,
			diff.WithWidth(m.width),
		)
		m.diffViewport.SetContent(diff)
		m.diffViewport.SetYOffset(starts - 1)
	} else {
		m.diffViewport.SetYOffset(0)
		m.diffViewport.SetContent("Select a comment to view diff context...")
	}

	return m
}

func (m Model) View() string {
	// Add header with comment count and help text
	header := m.renderHeader()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		m.commentsList.View(),
		m.diffViewport.View(),
	)
}

func fetchPRComments(ctx context.Context, client *github.Client, _ int) tea.Cmd {
	type ghPRView struct {
		Number         int `json:"number"`
		HeadRepository struct {
			Name string `json:"name"`
		} `json:"headRepository"`
		HeadRepositoryOwner struct {
			Login string `json:"login"`
		} `json:"headRepositoryOwner"`
	}
	return func() tea.Msg {
		log.Println("Fetching current PR number and repo info from gh CLI")
		// Get the current PR number using the gh CLI
		cmd := exec.Command("gh", "pr", "view", "--json", "number,headRepository,headRepositoryOwner")
		out, err := cmd.Output()
		log.Printf("out: %s", string(out))
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				log.Printf("gh CLI stderr: %s", string(exitErr.Stderr))
			}
			log.Printf("error running gh CLI: %v", err)
			return nil
		}

		// Parse the JSON output

		var prInfo ghPRView
		if err := json.Unmarshal(out, &prInfo); err != nil {
			log.Printf("error parsing PR info JSON: %v", err)
			return nil
		}

		prNumber := prInfo.Number
		owner := prInfo.HeadRepositoryOwner.Login
		repo := prInfo.HeadRepository.Name

		comments, _, err := client.PullRequests.ListComments(ctx, owner, repo, prNumber, nil)
		if err != nil {
			return err
		}
		return components.CommentsDataMsg{Comments: comments}
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
