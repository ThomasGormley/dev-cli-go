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

func handlePRCreate(stdout, stderr io.Writer, ghCli GitHubClienter) cli.ActionFunc {
	return func(c *cli.Context) error {
		if !isGitRepo() {
			return cli.Exit("Not a git repo", 1)
		}

		if err := ghCli.AuthStatus(); err != nil {
			return cli.Exit("Not authenticated with GitHub CLI, try running `gh auth login`", 1)
		}

		title, err := titleOrPrompt(c)

		if err != nil {
			return err
		}

		body, err := bodyOrPRTemplate(c)

		if err != nil {
			return err
		}

		base := c.String("base")
		if base == "" {
			stderr.Write([]byte("Base branch is required\n"))
		}

		if err := ghCli.CreatePR(title, body, base); err != nil {
			return cli.Exit(err, 1)
		}

		return cli.Exit("", 0)
	}
}

func handlePRView(stdout, stderr io.Writer, ghCli GitHubClienter) cli.ActionFunc {
	return func(c *cli.Context) error {
		identifier := c.Args().First()
		return ghCli.ViewPR(identifier)
	}
}

func bodyOrPRTemplate(c *cli.Context) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if body := c.String("body"); body == "" && isWorkstationDir(cwd) {
		body, err := firstupPRTemplate()
		if err != nil {
			return "", err
		}
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
