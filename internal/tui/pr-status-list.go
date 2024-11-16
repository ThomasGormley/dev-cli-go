package tui

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/thomasgormley/dev-cli-go/internal/gh"
)

type PullRequestStatus struct {
	mergeStateStatus gh.MergeStateStatus

	loaded     bool
	spinner    spinner.Model
	identifier string
	gh         gh.GitHubClienter
	list       list.Model
}

func NewPullRequestStatus(identifier string, gh gh.GitHubClienter) PullRequestStatus {
	l := list.New([]list.Item{}, NewDelegate(), 0, 0)
	l.SetShowTitle(false)
	return PullRequestStatus{
		spinner:    NewEllipsisSpinner(),
		list:       l,
		identifier: identifier,
		gh:         gh,
	}
}

func (m PullRequestStatus) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		CheckStatus(m.identifier, m.gh),
	)
}

func (m PullRequestStatus) View() string {
	if !m.loaded {
		return "checking CI status" + m.spinner.View()
	}
	// list
	return m.list.View()
}

func (m PullRequestStatus) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		log.Printf("pressing key: %s", msg.String())
		switch msg.String() {
		case " ", "enter":
			item, ok := m.list.SelectedItem().(statusCheckItem)
			if !ok {
				panic("not status check item?")
			}
			openBrowser(item.url)
			return m, nil
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	case tea.WindowSizeMsg:
		log.Printf("ini height %d, width %d", m.list.Height(), m.list.Width())
		if m.list.Width() == 0 && m.list.Height() == 0 {
			m.SetListSize(msg.Width, msg.Height)
		}
	case StatusCheckMsg:
		log.Println("received statusCheckMsg")
		// nothing to check
		m.loaded = true
		if len(msg.checks) == 0 && msg.MergeStateStatus == "" {
			return m, tea.Quit
		}
		log.Printf("loaded, setting items")
		return m, m.list.SetItems(msg.checks)
	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *PullRequestStatus) SetListSize(width int, height int) {
	log.Printf("Setting List Size Height %d, width %d", height, width)

	m.list.SetSize(width, height)
}

type StatusCheckMsg struct {
	checks           []list.Item
	MergeStateStatus gh.MergeStateStatus
	Title            string
	Base             string
	Head             string
	IsDraft          bool
	Closed           bool
}

func CheckStatus(identifier string, ghCli gh.GitHubClienter) tea.Cmd {
	return func() tea.Msg {
		status, err := ghCli.PRStatus(identifier)
		if err != nil {
			return CmdError{
				reason: err.Error(),
			}
		}
		if status.CurrentBranch.Closed {
			return CmdError{
				reason: "pull request already closed",
			}
		}
		checkItems := make([]list.Item, 0)
		for _, check := range status.CurrentBranch.StatusCheckRollup {
			checkItems = append(checkItems, statusCheckItem{
				name:       check.Name,
				context:    check.Context,
				conclusion: check.Conclusion,
				state:      check.State,
				status:     check.Status,
				url:        check.DetailsURL,
			})
		}
		return StatusCheckMsg{
			checks:           checkItems,
			MergeStateStatus: status.CurrentBranch.MergeStateStatus,
			Title:            status.CurrentBranch.Title,
			Base:             status.CurrentBranch.BaseRefName,
			Head:             status.CurrentBranch.HeadRefName,
			IsDraft:          status.CurrentBranch.IsDraft,
			Closed:           status.CurrentBranch.Closed,
		}
	}
}

type statusCheckItem struct {
	name       string
	context    string
	conclusion string
	state      string
	status     string
	url        string
}

func (i statusCheckItem) Title() string {
	if i.name == "" {
		return i.context
	}
	return i.name
}

func (i statusCheckItem) Description() string {
	desc := i.conclusion
	if desc == "" {
		desc = i.state
	}
	if desc == "" {
		desc = i.status
	}
	return withIcon(desc)
}

func (i statusCheckItem) FilterValue() string {
	if i.name == "" {
		return i.context
	}
	return i.name
}

func withIcon(s string) string {
	var icon string
	switch strings.ToLower(s) {
	case "success":
		icon = checkMark.String()
	case "skipped":
		icon = skipped.String()
	case "failure":
		icon = failure.String()
	default:
		icon = unknown.String()
	}
	return fmt.Sprintf("%s %s", icon, s)
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
