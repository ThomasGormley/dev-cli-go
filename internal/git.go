package cli

import (
	"github.com/thomasgormley/dev-cli-go/internal/git"
)

func isGitRepo() bool {
	return git.IsRepo()
}

func gitBranch() (string, error) {
	return git.CurrentBranch()
}

func repoPRTemplate() string {
	return git.GetPRTemplate()
}
