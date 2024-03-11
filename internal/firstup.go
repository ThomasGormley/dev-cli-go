package cli

import (
	"regexp"
	"strings"
)

// Directories that are used by the workstation CLI
const (
	socialchorusDirectory = "/socialchorus"
	firstupDirectory      = "/firstup"
)

var workstationDirs = []string{
	socialchorusDirectory,
	firstupDirectory,
}

func isWorkstationDir(dir string) bool {
	for _, d := range workstationDirs {
		if strings.Contains(dir, d) {
			return true
		}
	}
	return false
}

func firstupPRTemplate() (string, error) {
	template := repoPRTemplate()
	if template == nil {
		template = []byte(defaultPRTemplate)
	}

	branch, err := gitBranch()
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`([A-Z]+-\d+)`)
	ticket := re.FindString(branch)

	if ticket == "" {
		return string(template), nil
	}

	return string(applyJiraLinkToTemplate(template, ticket)), nil
}

func applyJiraLinkToTemplate(template []byte, ticket string) []byte {
	re := regexp.MustCompile(`(https:\/\/(firstup-io|socialcoders)\.atlassian\.net\/browse\/[A-Z]+-\d*)`)
	if !re.Match(template) {
		return template
	}
	return re.ReplaceAll(template, jiraURLForTicket(ticket))
}

func jiraURLForTicket(ticket string) []byte {
	return []byte("https://firstup-io.atlassian.net/browse/" + ticket)
}

const defaultPRTemplate = `**Issue**
https://firstup-io.atlassian.net/browse/FE-

**Changes**`

const firstupJiraRegex = `https:\/\/(firstup-io|socialcoders)\.atlassian\.net\/browse\/[A-Z]+-\d*`
