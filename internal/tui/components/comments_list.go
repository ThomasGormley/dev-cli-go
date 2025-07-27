package components

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/google/go-github/v74/github"
	"github.com/muesli/reflow/wordwrap"
	"github.com/thomasgormley/dev-cli-go/internal/tui/theme"
)

var _ tea.Model = &CommentsView{}

type CommentsView struct {
	Width, Height int
	blockCursor   int
	commentBlocks []commentBlock

	paginator       paginator.Model
	repliesViewport viewport.Model
}

type commentBlock struct {
	comment *github.PullRequestComment
	replies []*github.PullRequestComment
}

// CommentsSelectedMsg is sent when a comment is selected
type CommentsSelectedMsg struct {
	Comment *github.PullRequestComment
}

// CommentsDataMsg is used to update the comments list
type CommentsDataMsg struct {
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

	commentsList = func(w int) lipgloss.Style {
		return lipgloss.NewStyle().Width(w / 3).MaxWidth(w / 3)
	}
)

func NewCommentsList() CommentsView {
	return CommentsView{
		commentBlocks: []commentBlock{},

		paginator:       paginator.New(),
		repliesViewport: viewport.New(0, 0),
	}
}

func (c CommentsView) Init() tea.Cmd {
	return nil
}

func (c CommentsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			c = c.navigateUp()
			return c, selectedCommentMsg(c.GetCurrentComment())
		case "down", "j":
			c = c.navigateDown()
			return c, selectedCommentMsg(c.GetCurrentComment())
		case "o":
			return c, c.openInEditor()
		}
	case CommentsDataMsg:
		c.commentBlocks = buildCommentBlocks(msg.Comments)
		c.blockCursor = 0
		perPage := 3 // TODO calculate?
		totalPages := (len(c.commentBlocks) + perPage - 1) / perPage
		c.paginator = paginator.New(
			paginator.WithPerPage(perPage),
			paginator.WithTotalPages(totalPages),
		)
		c = c.ensureCursorBounds()
		return c, selectedCommentMsg(c.GetCurrentComment())
	}

	c.paginator, cmd = c.paginator.Update(msg)
	return c, cmd
}

func (c CommentsView) View() string {
	if len(c.commentBlocks) == 0 {
		return "No comments to display"
	}

	start, end := c.paginator.GetSliceBounds(len(c.commentBlocks))
	visibleItems := c.commentBlocks[start:end]

	var comments strings.Builder
	for blockIdx, item := range visibleItems {
		isFocused := c.blockCursor == blockIdx
		comments.WriteString(c.renderComment(item.comment, isFocused) + "\n")
	}
	comments.WriteString(" " + c.paginator.View())

	content := lipgloss.NewStyle().
		Height(c.Height).
		MaxHeight(c.Height)
	list := commentsList(c.Width)

	return content.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			list.Render(comments.String()),
			"",
		),
	)
}

// ensureCursorBounds ensures the cursor is within valid bounds for the current page
func (c CommentsView) ensureCursorBounds() CommentsView {
	if len(c.commentBlocks) == 0 {
		c.blockCursor = 0
		return c
	}

	start, end := c.paginator.GetSliceBounds(len(c.commentBlocks))
	visibleItems := end - start

	if visibleItems == 0 {
		c.blockCursor = 0
		return c
	}

	maxCursor := visibleItems - 1
	if c.blockCursor > maxCursor {
		c.blockCursor = maxCursor
	}
	if c.blockCursor < 0 {
		c.blockCursor = 0
	}

	return c
}

func (c CommentsView) navigateUp() CommentsView {
	if c.blockCursor > 0 {
		// Move up within current page
		c.blockCursor--
	} else if c.paginator.Page > 0 {
		// Move to previous page and position at last item
		c.paginator.PrevPage()
		start, end := c.paginator.GetSliceBounds(len(c.commentBlocks))
		visibleItems := end - start
		if visibleItems > 0 {
			c.blockCursor = visibleItems - 1
		}
	}
	// If at first item of first page, do nothing

	return c.ensureCursorBounds()
}

func (c CommentsView) openInEditor() tea.Cmd {
	return func() tea.Msg {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			return nil
		}
		parts := strings.Fields(editor)
		comment := c.GetCurrentComment()
		if comment == nil {
			return nil
		}
		repoRoot, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
		if err != nil {
			return err
		}
		repoRootPath := strings.TrimSpace(string(repoRoot))
		var lineNum int

		if isMultiline := comment.StartLine != nil; isMultiline {
			lineNum = comment.GetOriginalStartLine()
		} else {
			lineNum = comment.GetOriginalLine()
		}
		fullPath := fmt.Sprintf("%s/%s:%d", repoRootPath, comment.GetPath(), lineNum)
		cmd := exec.Command(parts[0], append(parts[1:], fullPath)...)
		return cmd.Run()
	}
}

func (c CommentsView) navigateDown() CommentsView {
	start, end := c.paginator.GetSliceBounds(len(c.commentBlocks))
	visibleItems := end - start

	if c.blockCursor < visibleItems-1 {
		// Move down within current page
		c.blockCursor++
	} else if c.paginator.Page < c.paginator.TotalPages-1 {
		// Move to next page and position at first item
		c.paginator.NextPage()
		c.blockCursor = 0
	}
	// If at last item of last page, do nothing

	return c.ensureCursorBounds()
}

func (c CommentsView) GetCurrentComment() *github.PullRequestComment {
	start, end := c.paginator.GetSliceBounds(len(c.commentBlocks))
	visibleItems := end - start

	if c.blockCursor >= visibleItems || start+c.blockCursor >= len(c.commentBlocks) {
		return nil
	}

	block := c.commentBlocks[start+c.blockCursor]
	return block.comment
}

func selectedCommentMsg(selected *github.PullRequestComment) tea.Cmd {
	return func() tea.Msg {
		return CommentsSelectedMsg{
			Comment: selected,
		}
	}
}

func (c CommentsView) renderComment(comment *github.PullRequestComment, isFocused bool) string {
	username := comment.GetUser().GetLogin()
	timestamp := comment.GetCreatedAt().Format("Jan 2, 15:04")

	// Format line numbers with side indicators (only for parent comments)
	var onLines string
	isMultiline := comment.StartLine != nil
	if !isMultiline {
		// Single line comment
		side := "+"
		sideStyle := addedSideStyle
		if comment.GetStartSide() == "LEFT" {
			side = "-"
			sideStyle = removedSideStyle
		}
		onLines = fmt.Sprintf("on line %s%d", sideStyle.Render(side), comment.GetOriginalLine())
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

		onLines = fmt.Sprintf("on lines %s%d to %s%d",
			startSideStyle.Render(startSide), comment.GetOriginalStartLine(),
			endSideStyle.Render(endSide), comment.GetOriginalLine())
	}
	body := strings.TrimSpace(comment.GetBody())
	if body == "" {
		body = "(empty comment)"
	}

	headerLine := fmt.Sprintf(
		"%s%s%s%s%s",
		usernameStyle.Render(fmt.Sprintf("[%s]", username)),
		dotStyle.Render(" • "),
		commentBodyStyle.Render(onLines),
		dotStyle.Render(" • "),
		timestampStyle.Render(timestamp),
	)
	bodyLine := commentBodyStyle.Render(body)

	border := lipgloss.HiddenBorder()
	if isFocused {
		border = lipgloss.RoundedBorder()
	}
	itemStyle := commentsList(c.Width).Border(border, true, false)

	content := headerLine + "\n" + bodyLine
	return wordwrap.String(itemStyle.Render(content), c.Width)
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

func buildCommentBlocks(comments []*github.PullRequestComment) []commentBlock {
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
	commentBlocks := make([]commentBlock, 0)
	for _, comment := range topLevel {
		block := commentBlock{
			comment: comment,
			replies: repliesMap[comment.GetID()],
		}
		commentBlocks = append(commentBlocks, block)
	}

	return commentBlocks
}
