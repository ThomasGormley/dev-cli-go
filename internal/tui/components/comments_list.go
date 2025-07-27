package components

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/google/go-github/v74/github"
	"github.com/thomasgormley/dev-cli-go/internal/tui/theme"
)

var _ tea.Model = &CommentsView{}

type CommentsView struct {
	width, height int
	blockCursor   int
	replyCursor   int
	commentBlocks []commentBlock
	focused       bool
}

type commentBlock struct {
	comment *github.PullRequestComment
	replies []*github.PullRequestComment
}

// CommentsSelectedMsg is sent when a comment is selected
type CommentsSelectedMsg struct {
	Comment *github.PullRequestComment
}

// CommentsUpdatedMsg is used to update the comments list
type CommentsUpdatedMsg struct {
	Comments []*github.PullRequestComment
}

var (
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

	// Side indicator styles for diff lines
	addedSideStyle = lipgloss.NewStyle().
			Foreground(theme.CurrentTheme().DiffAdded())

	removedSideStyle = lipgloss.NewStyle().
				Foreground(theme.CurrentTheme().DiffRemoved())
)

func NewCommentsList() CommentsView {
	return CommentsView{
		commentBlocks: []commentBlock{},
		focused:       true,
	}
}

func (c CommentsView) Init() tea.Cmd {
	return nil
}

func (c CommentsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !c.focused {
			return c, nil
		}

		switch msg.String() {
		case "up", "k":
			c = c.navigateUp()
			return c, c.emitSelectionChange()
		case "down", "j":
			c = c.navigateDown()
			return c, c.emitSelectionChange()
		}
	case tea.WindowSizeMsg:
		c.width, c.height = msg.Width, msg.Height
	case CommentsUpdatedMsg:
		c = c.buildCommentTree(msg.Comments)
		return c, c.emitSelectionChange()
	}
	return c, nil
}

func (c CommentsView) SetFocus(focused bool) {
	c.focused = focused
}

func (c CommentsView) IsFocused() bool {
	return c.focused
}

func (c CommentsView) buildCommentTree(comments []*github.PullRequestComment) CommentsView {
	// Separate top-level comments from replies
	var topLevel []*github.PullRequestComment
	repliesMap := make(map[int64][]*github.PullRequestComment)

	for _, comment := range comments {
		if parentID := comment.GetInReplyTo(); parentID > 0 {
			repliesMap[parentID] = append(repliesMap[parentID], comment)
		} else {
			topLevel = append(topLevel, comment)
		}
	}

	// Sort top-level comments by creation time
	sort.Slice(topLevel, func(i, j int) bool {
		return topLevel[i].GetCreatedAt().Before(topLevel[j].GetCreatedAt().Time)
	})

	// Sort replies for each parent by creation time
	for parentID := range repliesMap {
		sort.Slice(repliesMap[parentID], func(i, j int) bool {
			return repliesMap[parentID][i].GetCreatedAt().Before(repliesMap[parentID][j].GetCreatedAt().Time)
		})
	}

	// Build comment blocks
	c.commentBlocks = nil
	for _, comment := range topLevel {
		block := commentBlock{
			comment: comment,
			replies: repliesMap[comment.GetID()],
		}
		c.commentBlocks = append(c.commentBlocks, block)
	}

	// Reset cursor position
	c.blockCursor = 0
	c.replyCursor = 0

	return c
}

func (c CommentsView) navigateUp() CommentsView {
	if c.blockCursor == 0 && c.replyCursor == 0 {
		return c // Already at the top
	}

	if c.replyCursor > 0 {
		c.replyCursor--
	} else {
		// Move to previous block
		if c.blockCursor > 0 {
			c.blockCursor--
			// Position at the last reply of the previous block, or main comment if no replies
			if len(c.commentBlocks[c.blockCursor].replies) > 0 {
				c.replyCursor = len(c.commentBlocks[c.blockCursor].replies)
			} else {
				c.replyCursor = 0
			}
		}
	}

	return c
}

func (c CommentsView) navigateDown() CommentsView {
	if c.blockCursor >= len(c.commentBlocks) {
		return c
	}

	maxReplies := len(c.commentBlocks[c.blockCursor].replies)

	if c.replyCursor < maxReplies {
		c.replyCursor++
	} else {
		// Move to next block
		if c.blockCursor < len(c.commentBlocks)-1 {
			c.blockCursor++
			c.replyCursor = 0
		}
	}

	return c
}

func (c CommentsView) GetCurrentComment() *github.PullRequestComment {
	if c.blockCursor >= len(c.commentBlocks) {
		return nil
	}

	block := c.commentBlocks[c.blockCursor]
	if c.replyCursor == 0 {
		return block.comment
	} else if c.replyCursor <= len(block.replies) {
		return block.replies[c.replyCursor-1]
	}

	return nil
}

func (c CommentsView) emitSelectionChange() tea.Cmd {
	return func() tea.Msg {
		return CommentsSelectedMsg{
			Comment: c.GetCurrentComment(),
		}
	}
}

func (c CommentsView) renderComment(comment *github.PullRequestComment, isSelected bool, isReply bool) string {
	cursor := " "
	if isSelected {
		cursor = "▶"
	}

	username := comment.GetUser().GetLogin()
	timestamp := comment.GetCreatedAt().Format("Jan 2, 15:04")

	// Format line numbers with side indicators (only for parent comments)
	var onLines string
	if !isReply {
		if comment.GetStartLine() == comment.GetLine() {
			// Single line comment
			side := "+"
			sideStyle := addedSideStyle
			if comment.GetStartSide() == "LEFT" {
				side = "-"
				sideStyle = removedSideStyle
			}
			if isSelected {
				sideStyle = sideStyle.Background(selectedStyle.GetBackground())
			}
			onLines = fmt.Sprintf("on line %s%d", sideStyle.Render(side), comment.GetStartLine())
		} else {
			// Multi-line comment
			startSide := "+"
			startSideStyle := addedSideStyle
			if comment.GetStartSide() == "LEFT" {
				startSide = "-"
				startSideStyle = removedSideStyle
			}

			endSide := "+"
			endSideStyle := addedSideStyle
			if comment.GetSide() == "LEFT" {
				endSide = "-"
				endSideStyle = removedSideStyle
			}

			if isSelected {
				startSideStyle = startSideStyle.Background(selectedStyle.GetBackground())
				endSideStyle = endSideStyle.Background(selectedStyle.GetBackground())
			}

			onLines = fmt.Sprintf("on lines %s%d to %s%d",
				startSideStyle.Render(startSide), comment.GetStartLine(),
				endSideStyle.Render(endSide), comment.GetLine())
		}
	}
	body := strings.TrimSpace(comment.GetBody())

	if body == "" {
		body = "(empty comment)"
	}

	// Format header line: [Username] • Timestamp
	styledUsername := usernameStyle.Render(fmt.Sprintf("[%s]", username))
	dot := dotStyle.Render(" • ")
	styledTimestamp := timestampStyle.Render(timestamp)

	var headerLine string
	if !isReply {
		headerLine = fmt.Sprintf("%s %s%s%s\n%s",
			cursor,
			styledUsername,
			dot,
			styledTimestamp, onLines)
	} else {
		headerLine = fmt.Sprintf("%s %s%s%s",
			cursor,
			styledUsername,
			dot,
			styledTimestamp)
	}

	// Format comment body
	styledBody := commentBodyStyle.Render(body)
	bodyLine := styledBody

	// Apply selection styling if this is the current item
	if isSelected {
		headerLine = selectedStyle.Render(headerLine)
		bodyLine = selectedStyle.Render(bodyLine)
	}

	result := headerLine + "\n" + bodyLine + "\n\n"

	// Apply padding for replies
	if isReply {
		replyStyle := lipgloss.NewStyle()
		result = replyStyle.Render(result)
	}

	return result
}

func (c CommentsView) View() string {
	if len(c.commentBlocks) == 0 {
		return "No comments to display"
	}

	var commentsList string

	for blockIdx, block := range c.commentBlocks {
		// Render main comment
		isSelected := c.blockCursor == blockIdx && c.replyCursor == 0
		commentsList += c.renderComment(block.comment, isSelected, false)

		// Render replies
		for replyIdx, reply := range block.replies {
			isSelected := c.blockCursor == blockIdx && c.replyCursor == replyIdx+1
			commentsList += c.renderComment(reply, isSelected, true)
		}

		// Add spacing between comment blocks
		if blockIdx < len(c.commentBlocks)-1 {
			commentsList += "\n"
		}
	}

	return commentsList
}

func (c CommentsView) GetStats() (total, topLevel, replies int) {
	topLevel = len(c.commentBlocks)
	replies = 0

	for _, block := range c.commentBlocks {
		replies += len(block.replies)
	}

	total = topLevel + replies
	return
}
