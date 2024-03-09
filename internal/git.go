package cli

import (
	"errors"
	"os"
	"os/exec"
)

func isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	return cmd.Run() == nil
}

func isAuthenticated() bool {
	cmd := exec.Command("gh", "auth", "status")
	return cmd.Run() == nil
}

func gitBranch() (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	out, err := cmd.Output()
	return string(out), err
}

var gitPullRequestTemplatePaths = []string{
	"./",
	"./.github/PULL_REQUEST_TEMPLATE/",
	"./.github/",
	"./docs/",
}

const pullRequestTemplateFilename = "PULL_REQUEST_TEMPLATE.md"

func pullRequestTemplate() ([]byte, error) {
	root, err := gitRoot()

	if err != nil {
		return nil, err
	}

	for _, p := range gitPullRequestTemplatePaths {
		path := root + p + pullRequestTemplateFilename
		if _, err := os.Stat(path); err == nil {
			return os.ReadFile(path)
		}
	}

	return nil, errors.New("no pull request template found")
}

func gitRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	return string(out), err
}
