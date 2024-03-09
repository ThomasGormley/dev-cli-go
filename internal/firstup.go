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
	t, err := pullRequestTemplate()
	if err != nil {
		return "", err
	}

	if t == nil {
		return defaultPRTemplate, nil
	}
	branch, err := gitBranch()
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`^([A-Z]+-\d+)-(.*)`)
	ticket := re.Find([]byte(branch))

	if ticket == nil {
		return string(t), nil
	}

	ticketStr := string(ticket)
	hasJiraLink := regexp.MustCompile(firstupJiraRegex).Match([]byte(string(t)))
	if hasJiraLink {
		withJira := re.ReplaceAll(ticket, jiraURLForTicket(ticketStr))
		return string(withJira), nil
	}

	return string(t), nil
}

func jiraURLForTicket(ticket string) []byte {
	return []byte("https://firstup-io.atlassian.net/browse/" + ticket)
}

const defaultPRTemplate = `**Issue**
https://firstup-io.atlassian.net/browse/FE-

**Changes**`

const firstupJiraRegex = `/https:\/\/(firstup-io|socialcoders)\.atlassian\.net\/browse\/[A-Z]+-/`
