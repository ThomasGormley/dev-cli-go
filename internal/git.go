package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path"
)

func isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	return cmd.Run() == nil
}

func gitBranch() (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	out, err := cmd.Output()
	return string(bytes.TrimSpace(out)), err
}

var gitPullRequestTemplatePaths = []string{
	"./",
	"./.github/PULL_REQUEST_TEMPLATE/",
	"./.github/",
	"./docs/",
}

const pullRequestTemplateFilename = "PULL_REQUEST_TEMPLATE.md"

func repoPRTemplate() []byte {
	root, err := gitRoot()

	if err != nil {
		return nil
	}

	for _, p := range gitPullRequestTemplatePaths {
		path := path.Join(root, p, pullRequestTemplateFilename)
		if _, err := os.Stat(path); err == nil {
			file, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			return file
		}
	}
	return nil
}

func gitRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(out)), nil
}
