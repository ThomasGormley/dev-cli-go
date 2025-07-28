package cli

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/thomasgormley/dev-cli-go/internal/gh"
	"github.com/urfave/cli/v2"
)

func handlePRCreate(stdout, stderr io.Writer, ghCli gh.GitHubClienter) cli.ActionFunc {
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

		if err := ghCli.CreatePR(title, body, base, c.Bool("draft")); err != nil {
			return cli.Exit(err, 1)
		}

		return cli.Exit("", 0)
	}
}

func handlePRView(stdout, stderr io.Writer, ghCli gh.GitHubClienter) cli.ActionFunc {
	return func(c *cli.Context) error {
		identifier := c.Args().First()
		return ghCli.ViewPR(identifier)
	}
}

func bodyOrPRTemplate(c *cli.Context) (string, error) {
	body := c.String("body")
	if body == "" {
		body = repoPRTemplate()
	}
	return body, nil
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
	// e.g. ABC-123-some-description or anystring-ABC-123-some-description
	// -> ABC-123: Some description
	re := regexp.MustCompile(`^(?:[a-zA-Z0-9]+-)?([a-zA-Z]+-\d+)-([a-z0-9-]+)$`)
	matches := re.FindStringSubmatch(branch)

	if len(matches) < 3 {
		return ""
	}

	t, d := strings.ToUpper(matches[1]), matches[2]
	d = strings.ReplaceAll(d, "-", " ")
	if t == "" {
		return d
	}

	return fmt.Sprintf("%s: %s", t, d)
}
