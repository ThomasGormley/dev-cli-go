package cli

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/urfave/cli/v2"
)

func handlePRCreate(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		stdout.Write([]byte("Creating a new pull request\n"))
		if !isGitRepo() {
			writeError(stderr, "Not a git repository\n")
			return nil
		}

		if !isAuthenticated() {
			stderr.Write([]byte("Not authenticated\n"))
			return fmt.Errorf("not authenticated")
		}

		title, err := titleOrPrompt(c)

		if err != nil {
			writeError(stderr, "Error getting title: %v\n", err)
			return nil
		}

		body, err := handleBody(c)

		base := c.String("base")
		if base == "" {
			// TODO: prompt for base
			stderr.Write([]byte("Base branch is required\n"))
		}

		ghArgs := []string{"pr", "create",
			"--title", wrapWithQuotes(title),
			"--body", wrapWithQuotes(body),
			"--base", wrapWithQuotes(base),
		}

		fmt.Printf("Running gh with args: %v\n", ghArgs)
		return nil
	}
}

func wrapWithQuotes(s string) string {
	return fmt.Sprintf(`"%s"`, s)
}

func handleBody(c *cli.Context) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if body := c.String("body"); body == "" && isWorkstationDir(cwd) {
		c.Set("body", body)
	}
	return c.String("body"), nil
}

func titleOrPrompt(c *cli.Context) (string, error) {
	title := c.String("title")
	if title == "" {
		title, err := promptForTitle()
		if err != nil {
			return "", err
		}
		c.Set("title", title)
	}
	return c.String("title"), nil
}

func promptForTitle() (string, error) {
	branch, err := gitBranch()
	if err != nil {
		return "", err
	}

	suggestedTitle := prTitleFromBranch(branch)

	prompt := &survey.Input{
		Message: "Title",
		Default: suggestedTitle,
	}
	var title string
	err = survey.AskOne(prompt, &title)
	return title, err
}

func prTitleFromBranch(branch string) string {
	// e.g. ABC-123-some-description
	// -> ABC-123: Some description
	re := regexp.MustCompile(`^([A-Z]+-\d+)-(.*)`)
	matches := re.FindStringSubmatch(branch)

	if len(matches) < 3 {
		return ""
	}

	t, d := matches[1], matches[2]
	d = strings.ReplaceAll(d, "-", " ")
	if t == "" {
		return d
	}

	return fmt.Sprintf("%s: %s", t, d)
}
