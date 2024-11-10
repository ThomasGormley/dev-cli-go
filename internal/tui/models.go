package tui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/huh"
)

type MergeModel struct {
	// mergeStateStatus cli.MergeStateStatus
	merged bool
	// strategy         cli.MergeStrategy
	messages   []string
	spinner    spinner.Model
	list       list.Model
	form       *huh.Form
	identifier string
	// ghClient         cli.GitHubClienter

	width, height int
}
