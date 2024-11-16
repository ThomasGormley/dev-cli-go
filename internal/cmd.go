package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/thomasgormley/dev-cli-go/internal/gh"
)

type mergeCmd string

func awaitMerge(strategy gh.MergeStrategy, ghCli gh.GitHubClienter) tea.Cmd {
	return func() tea.Msg {
		err := ghCli.MergePR(strategy)
		if err != nil {
			// return err command
			return nil
		}
		return mergeCmd("")
	}
}

type statusCheckCmd struct {
	checks           []list.Item
	mergeStateStatus gh.MergeStateStatus
	title            string
	base             string
	head             string
}

func awaitStatusCheckCmd(identifier string, ghCli gh.GitHubClienter) tea.Cmd {
	return func() tea.Msg {
		status, err := ghCli.PRStatus(identifier)
		if err != nil {
			return tea.Printf("error getting status: %+v", err)
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
		return statusCheckCmd{
			checks:           checkItems,
			mergeStateStatus: status.CurrentBranch.MergeStateStatus,
			title:            status.CurrentBranch.Title,
			base:             status.CurrentBranch.BaseRefName,
			head:             status.CurrentBranch.HeadRefName,
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
	if i.conclusion == "" {
		desc = i.state
	} else if i.state == "" {
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
	}
	return fmt.Sprintf("%s %s", icon, s)
}
