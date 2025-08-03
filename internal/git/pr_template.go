package git

import (
	"os"
	"path"
)

var pullRequestTemplatePaths = []string{
	"./",
	"./.github/PULL_REQUEST_TEMPLATE/",
	"./.github/",
	"./docs/",
}

const pullRequestTemplateFilename = "PULL_REQUEST_TEMPLATE.md"

// GetPRTemplate returns the content of the pull request template if found
func GetPRTemplate() string {
	root, err := Root()
	if err != nil {
		return ""
	}

	for _, p := range pullRequestTemplatePaths {
		templatePath := path.Join(root, p, pullRequestTemplateFilename)
		if _, err := os.Stat(templatePath); err == nil {
			file, err := os.ReadFile(templatePath)
			if err != nil {
				return ""
			}
			return string(file)
		}
	}
	return ""
}
