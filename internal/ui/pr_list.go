package ui

import (
	"fmt"
	"io"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/thomasgormley/dev-cli-go/internal/gh"
	"github.com/thomasgormley/dev-cli-go/internal/git"
	"github.com/thomasgormley/dev-cli-go/internal/serve"
)

// Color palette for PR list component
const (
	colorNeutral    = "#e1e2e7"
	colorPrimary    = "#2e7de9"
	colorSuccess    = "#587539"
	colorWarning    = "#8c6c3e"
	colorError      = "#c94060"
	colorDiffAdd    = "#4f8f7b"
	colorDiffDelete = "#d05f7c"
	colorDimmed     = "#6c7086"
)

type PRListKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Open     key.Binding
	Checkout key.Binding
	Dispatch key.Binding
	Quit     key.Binding
}

func DefaultPRListKeyMap() PRListKeyMap {
	return PRListKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "ctrl+k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "ctrl+j"),
			key.WithHelp("↓/j", "down"),
		),
		Open: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open"),
		),
		Checkout: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "checkout"),
		),
		Dispatch: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "dispatch"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc"),
			key.WithHelp("q/esc", "quit"),
		),
	}
}

type PRListStyle struct {
	Title       lipgloss.Style
	Selected    lipgloss.Style
	Description lipgloss.Style
	Spinner     lipgloss.Style
	Empty       lipgloss.Style
	Success     lipgloss.Style
	Error       lipgloss.Style

	// List component styles
	ListStyles          list.Styles
	ListItemStyles      list.DefaultItemStyles
	ListItemAddStyle    lipgloss.Style
	ListItemDeleteStyle lipgloss.Style
}

type palette struct {
	neutral, primary, success, warning, errorColor lipgloss.Color
	diffAdd, diffDelete, dimmed                    lipgloss.Color
}

func DefaultPRListStyle() PRListStyle {
	c := palette{
		neutral:    lipgloss.Color("#e1e2e7"),
		primary:    lipgloss.Color("#2e7de9"),
		success:    lipgloss.Color("#587539"),
		warning:    lipgloss.Color("#8c6c3e"),
		errorColor: lipgloss.Color("#c94060"),
		diffAdd:    lipgloss.Color("#4f8f7b"),
		diffDelete: lipgloss.Color("#d05f7c"),
		dimmed:     lipgloss.Color("#6c7086"),
	}

	listStyles := list.DefaultStyles()
	listStyles.Title = lipgloss.NewStyle().
		Background(c.primary).Foreground(c.neutral).Padding(0, 1)
	listStyles.StatusBar = lipgloss.NewStyle().
		Foreground(c.warning).Padding(0, 0, 1, 2)
	listStyles.StatusEmpty = lipgloss.NewStyle().Foreground(c.warning)
	listStyles.StatusBarActiveFilter = lipgloss.NewStyle().Foreground(c.neutral)

	itemStyles := list.NewDefaultItemStyles()
	itemStyles.NormalTitle = itemStyles.NormalTitle.Foreground(c.neutral)
	itemStyles.NormalDesc = itemStyles.NormalDesc.Foreground(c.warning)
	itemStyles.SelectedTitle = itemStyles.SelectedTitle.
		Foreground(c.primary).BorderForeground(c.primary)
	itemStyles.SelectedDesc = itemStyles.SelectedTitle.Foreground(c.neutral)
	itemStyles.DimmedTitle = itemStyles.DimmedTitle.Foreground(c.dimmed)
	itemStyles.DimmedDesc = itemStyles.DimmedDesc.Foreground(c.dimmed)

	return PRListStyle{
		Title:               lipgloss.NewStyle().Bold(true),
		Selected:            lipgloss.NewStyle().Foreground(c.primary),
		Description:         lipgloss.NewStyle().Foreground(c.neutral),
		Spinner:             lipgloss.NewStyle().Foreground(c.primary),
		Empty:               lipgloss.NewStyle().Foreground(c.neutral).Italic(true),
		Success:             lipgloss.NewStyle().Foreground(c.success),
		Error:               lipgloss.NewStyle().Foreground(c.errorColor),
		ListItemAddStyle:    lipgloss.NewStyle().Foreground(c.diffAdd),
		ListItemDeleteStyle: lipgloss.NewStyle().Foreground(c.diffDelete),
		ListStyles:          listStyles,
		ListItemStyles:      itemStyles,
	}
}

type PRListItem struct {
	pr gh.PullRequest
}

func (i PRListItem) Title() string {
	if i.pr.IsDraft {
		return i.pr.Title + " (draft)"
	}
	return i.pr.Title
}

func (i PRListItem) Description() string {
	return fmt.Sprintf("%s → %s  +%d -%d",
		i.pr.HeadRefName,
		i.pr.BaseRefName,
		i.pr.Additions,
		i.pr.Deletions)
}

func (i PRListItem) FilterValue() string {
	return i.pr.Title
}

type prListDelegate struct {
	styles   list.DefaultItemStyles
	addStyle lipgloss.Style
	delStyle lipgloss.Style
	height   int
	spacing  int
}

func newPRListDelegate(style PRListStyle) prListDelegate {
	return prListDelegate{
		styles:   style.ListItemStyles,
		addStyle: style.ListItemAddStyle,
		delStyle: style.ListItemDeleteStyle,
		height:   2,
		spacing:  0,
	}
}

func (d *prListDelegate) SetHeight(i int) {
	d.height = i
}

func (d *prListDelegate) SetSpacing(i int) {
	d.spacing = i
}

func (d prListDelegate) Height() int  { return d.height }
func (d prListDelegate) Spacing() int { return d.spacing }
func (d prListDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d prListDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(PRListItem)
	if !ok {
		return
	}

	if m.Width() <= 0 {
		return
	}

	title := i.Title()
	branches := fmt.Sprintf("%s → %s", i.pr.HeadRefName, i.pr.BaseRefName)
	adds := fmt.Sprintf("+%d", i.pr.Additions)
	dels := fmt.Sprintf("-%d", i.pr.Deletions)

	textWidth := m.Width() - d.styles.NormalTitle.GetPaddingLeft() - d.styles.NormalTitle.GetPaddingRight()
	title = ansi.Truncate(title, textWidth, "…")

	isSelected := index == m.Index()
	emptyFilter := m.FilterState() == list.Filtering && m.FilterValue() == ""
	isFiltered := m.FilterState() == list.Filtering || m.FilterState() == list.FilterApplied

	var matchedRunes []int
	if isFiltered {
		matchedRunes = m.MatchesForItem(index)
	}

	var titleStr, descStr string
	if emptyFilter {
		titleStr = d.styles.DimmedTitle.Render(title)
		descStr = d.styles.DimmedDesc.Render(branches) + "  " +
			d.addStyle.Render(adds) + " " +
			d.delStyle.Render(dels)
	} else if isSelected && m.FilterState() != list.Filtering {
		if isFiltered {
			unmatched := d.styles.SelectedTitle.Inline(true)
			matched := unmatched.Inherit(d.styles.FilterMatch)
			title = lipgloss.StyleRunes(title, matchedRunes, matched, unmatched)
		}
		titleStr = d.styles.SelectedTitle.Render(title)
		descStr = d.styles.SelectedDesc.Render(branches) + "  " +
			d.addStyle.Render(adds) + " " +
			d.delStyle.Render(dels)
	} else {
		if isFiltered {
			unmatched := d.styles.NormalTitle.Inline(true)
			matched := unmatched.Inherit(d.styles.FilterMatch)
			title = lipgloss.StyleRunes(title, matchedRunes, matched, unmatched)
		}
		titleStr = d.styles.NormalTitle.Render(title)
		descStr = d.styles.NormalDesc.Render(branches) + "  " +
			d.addStyle.Render(adds) + " " +
			d.delStyle.Render(dels)
	}

	fmt.Fprintf(w, "%s\n%s", titleStr, descStr) //nolint:errcheck
}

type authCheckMsg struct {
	err error
}

type prsLoadedMsg struct {
	prs []gh.PullRequest
	err error
}

type actionDoneMsg struct {
	action string
	err    error
	exit   bool
}

type loadingState int

const (
	stateCheckAuth loadingState = iota
	stateFetchPRs
	stateReady
)

type PRListModel struct {
	KeyMap   PRListKeyMap
	Style    PRListStyle
	ghClient gh.GitHubClienter

	spinner spinner.Model
	list    list.Model

	state  loadingState
	prs    []gh.PullRequest
	err    error
	action string
	done   bool
	width  int
	height int
}

func NewPRList(ghClient gh.GitHubClienter, serveClient serve.Client) PRListModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return PRListModel{
		KeyMap:   DefaultPRListKeyMap(),
		Style:    DefaultPRListStyle(),
		ghClient: ghClient,
		spinner:  sp,
		state:    stateCheckAuth,
	}
}

func (m PRListModel) WithKeyMap(km PRListKeyMap) PRListModel {
	m.KeyMap = km
	return m
}

func (m PRListModel) WithStyle(s PRListStyle) PRListModel {
	m.Style = s
	return m
}

func (m PRListModel) Error() error {
	return m.err
}

func authCheckCmd(client gh.GitHubClienter) tea.Cmd {
	return func() tea.Msg {
		err := client.AuthStatus()
		return authCheckMsg{err: err}
	}
}

func fetchPRsCmd(client gh.GitHubClienter) tea.Cmd {
	return func() tea.Msg {
		prs, err := client.ListPRs()
		return prsLoadedMsg{prs: prs, err: err}
	}
}

func openPRCmd(client gh.GitHubClienter, number int) tea.Cmd {
	return func() tea.Msg {
		err := client.ViewPR(fmt.Sprintf("%d", number))
		return actionDoneMsg{action: "open", err: err, exit: false}
	}
}

func checkoutCmd(branch string) tea.Cmd {
	return func() tea.Msg {
		err := git.Checkout(branch)
		return actionDoneMsg{action: "checkout", err: err}
	}
}

func dispatchCmd(client gh.GitHubClienter, number int) tea.Cmd {
	return func() tea.Msg {
		err := client.ViewPR(fmt.Sprintf("%d", number))
		return actionDoneMsg{action: "dispatch", err: err}
	}
}

func (m PRListModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		authCheckCmd(m.ghClient),
	)
}

func (m PRListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.state == stateReady {
			m.list.SetSize(msg.Width, msg.Height)
		}
		return m, nil

	case authCheckMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.state = stateFetchPRs
		return m, fetchPRsCmd(m.ghClient)

	case prsLoadedMsg:
		m.state = stateReady
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.prs = msg.prs

		items := make([]list.Item, len(msg.prs))
		for i, pr := range msg.prs {
			items[i] = PRListItem{pr: pr}
		}

		delegate := newPRListDelegate(m.Style)
		m.list = list.New(items, delegate, m.width, m.height)
		m.list.Title = "Pull Requests"
		m.list.Styles = m.Style.ListStyles
		m.list.SetShowHelp(true)
		m.list.StatusMessageLifetime = 3 * time.Second
		m.list.AdditionalShortHelpKeys = func() []key.Binding {
			return []key.Binding{
				m.KeyMap.Open,
				m.KeyMap.Checkout,
				m.KeyMap.Dispatch,
			}
		}

		return m, nil

	case actionDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.action = msg.action
			m.done = true
			return m, tea.Quit
		}
		if msg.exit {
			m.done = true
			return m, tea.Quit
		}
		return m, nil

	case tea.KeyMsg:
		if m.state != stateReady {
			if key.Matches(msg, m.KeyMap.Quit) {
				return m, tea.Quit
			}
			return m, nil
		}

		if m.list.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}

		switch {
		case key.Matches(msg, m.KeyMap.Open):
			if item, ok := m.list.SelectedItem().(PRListItem); ok {
				statusCmd := m.list.
					NewStatusMessage(
						m.Style.Title.Render("Opened PR #" + fmt.Sprintf("%d", item.pr.Number)),
					)
				return m, tea.Batch(openPRCmd(m.ghClient, item.pr.Number), statusCmd)
			}
		case key.Matches(msg, m.KeyMap.Checkout):
			if item, ok := m.list.SelectedItem().(PRListItem); ok {
				statusCmd := m.list.
					NewStatusMessage(
						m.Style.Title.Render("Checked out branch '" + item.pr.HeadRefName + "'"),
					)
				return m, tea.Batch(checkoutCmd(item.pr.HeadRefName), statusCmd)
			}
		case key.Matches(msg, m.KeyMap.Dispatch):
			if item, ok := m.list.SelectedItem().(PRListItem); ok {
				return m, dispatchCmd(m.ghClient, item.pr.Number)
			}
		case key.Matches(msg, m.KeyMap.Quit):
			return m, tea.Quit
		}

		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd

	default:
		if m.state != stateReady {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
}

func (m PRListModel) View() string {
	if m.done {
		if m.err != nil {
			return m.Style.Error.Render(fmt.Sprintf("Error: %v", m.err))
		}
		return m.Style.Success.Render(fmt.Sprintf("%s complete", m.action))
	}

	switch m.state {
	case stateCheckAuth:
		return m.Style.Spinner.Render(fmt.Sprintf("%s Checking GitHub authentication...", m.spinner.View()))
	case stateFetchPRs:
		return m.Style.Spinner.Render(fmt.Sprintf("%s Fetching pull requests...", m.spinner.View()))
	case stateReady:
		if len(m.prs) == 0 {
			return m.Style.Empty.Render("No open pull requests found")
		}
		return m.list.View()
	}

	return ""
}
