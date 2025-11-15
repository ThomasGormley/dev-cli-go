package cli

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/thomasgormley/dev-cli-go/internal/gh"
	"github.com/thomasgormley/dev-cli-go/internal/print"
	"github.com/thomasgormley/dev-cli-go/internal/spinner"
	"github.com/urfave/cli/v2"
)

func handlePRCreate(stdout, stderr io.Writer, ghCli gh.GitHubClienter) cli.ActionFunc {
	return func(c *cli.Context) error {
		if !isGitRepo() {
			print.Error(stderr, "Not a git repository")
			return cli.Exit("", 1)
		}

		if err := spinner.With("Checking GitHub authentication.", func() error {
			return ghCli.AuthStatus()
		}, spinner.WithWriter(stdout), spinner.WithFailureMessage("Not authenticated with GitHub CLI"), spinner.WithSuccessMessage("Authenticated")); err != nil {
			print.Info(stdout, print.WrapBottom(
				print.ColorNote("Run 'gh auth login' to authenticate"),
			))
			return cli.Exit("", 1)
		}

		prStatus, err := ghCli.PRStatus("")
		if err != nil {
			// non critical error, just continue
		}

		if !prStatus.CurrentBranch.Closed {
			print.Info(stdout,
				print.ColorNote(print.InfoSym),
				"Pull request already exists for this branch",
			)
			print.Info(stdout, print.WrapTop(
				print.ColorNote("Title:"),
				prStatus.CurrentBranch.Title,
			))
			print.Info(stdout, print.WrapBottom(
				print.ColorNote("URL:"),
				prStatus.CurrentBranch.URL,
			))

			if err := promptForOpen(stdout, ghCli); err != nil {
				print.Warning(stdout, print.WarningSym, "Could not open PR in browser")
			}
			return cli.Exit("", 0)
		}

		title, err := titleOrPrompt(c)
		if err != nil {
			return err
		}

		body, err := bodyOrPRTemplate(c)
		if err != nil {
			return err
		}

		// Show info about what we're using
		base := c.String("base")
		draft := c.Bool("draft")
		if body == "" {
			print.Info(stdout, print.WrapBottom(
				print.InfoSym,
				"No body provided, using PR template",
			))
		}
		message := "Creating pull request"
		if draft {
			message = "Creating draft pull request"
		}

		err = spinner.With(message, func() error {
			return ghCli.CreatePR(title, body, base, draft)
		}, spinner.WithWriter(stdout), spinner.WithFailureMessage("Failed to create pull request"), spinner.WithSuccessMessage("Pull request created successfully"))
		if err != nil {
			return cli.Exit("", 1)
		}

		if err := promptForOpen(stdout, ghCli); err != nil {
			print.Warning(stdout, print.Wrap(
				print.WarningSym,
				"Could not open PR in browser",
			))
		}

		return cli.Exit("", 0)
	}
}

func handlePRView(stdout, stderr io.Writer, ghCli gh.GitHubClienter) cli.ActionFunc {
	return func(c *cli.Context) error {
		identifier := c.Args().First()

		if identifier == "" {
			branch, err := gitBranch()
			if err != nil {
				branch = "current branch"
			}
			print.Info(stdout, print.Wrap(
				"Viewing PR for branch:",
				print.ColorNote(branch),
			))
			return ghCli.ViewPR(identifier)
		}

		print.Info(stdout, print.Wrap(
			print.ColorNote("Viewing PR:"),
			identifier,
		))
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

func promptForOpen(stdout io.Writer, ghCli gh.GitHubClienter) error {
	var openInBrowser bool
	prompt := &survey.Confirm{
		Message: "Open pull request in browser?",
		Default: true,
	}

	if err := survey.AskOne(prompt, &openInBrowser); err != nil {
		return err
	}

	if openInBrowser {
		print.Info(stdout, print.Wrap(
			print.Arrow,
			"Opening pull request in browser...",
		))
		return ghCli.ViewPR("")
	}

	return nil
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
