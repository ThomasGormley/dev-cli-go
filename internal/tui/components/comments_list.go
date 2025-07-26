package components

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/go-github/v74/github"
)

var _ tea.Model = &CommentsList{}

type CommentsList struct {
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
)

func NewCommentsList() CommentsList {
	return CommentsList{
		commentBlocks: []commentBlock{},
		focused:       true,
	}
}

func (c CommentsList) Init() tea.Cmd {
	return nil
}

func (c CommentsList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (c CommentsList) SetFocus(focused bool) {
	c.focused = focused
}

func (c CommentsList) IsFocused() bool {
	return c.focused
}

func (c CommentsList) buildCommentTree(comments []*github.PullRequestComment) CommentsList {
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

func (c CommentsList) navigateUp() CommentsList {
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

func (c CommentsList) navigateDown() CommentsList {
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

func (c CommentsList) GetCurrentComment() *github.PullRequestComment {
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

func (c CommentsList) emitSelectionChange() tea.Cmd {
	return func() tea.Msg {
		return CommentsSelectedMsg{
			Comment: c.GetCurrentComment(),
		}
	}
}

func (c CommentsList) renderComment(comment *github.PullRequestComment, isSelected bool, isReply bool) string {
	cursor := " "
	if isSelected {
		cursor = "▶"
	}

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

func (c CommentsList) View() string {
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

func (c CommentsList) GetStats() (total, topLevel, replies int) {
	topLevel = len(c.commentBlocks)
	replies = 0

	for _, block := range c.commentBlocks {
		replies += len(block.replies)
	}

	total = topLevel + replies
	return
}
