package cli

import (
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/thomasgormley/dev-cli-go/internal/clipboard"
	"github.com/thomasgormley/dev-cli-go/internal/gh"
	"github.com/thomasgormley/dev-cli-go/internal/git"
	"github.com/thomasgormley/dev-cli-go/internal/print"
	"github.com/thomasgormley/dev-cli-go/internal/spinner"
	"github.com/urfave/cli/v2"
)

func handlePRCreate(stdout, stderr io.Writer, ghCli gh.GitHubClienter) cli.ActionFunc {
	return func(c *cli.Context) error {
		if err := ensurePRContext(stdout, stderr, ghCli); err != nil {
			return err
		}

		prStatus, err := ghCli.PRStatus("")
		if err != nil {
			// non critical error, just continue
		}

		if prStatus.CurrentBranch.URL != "" && !prStatus.CurrentBranch.Closed {
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
				return err
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
			return err
		}

		return cli.Exit("", 0)
	}
}

func handlePRView(stdout, stderr io.Writer, ghCli gh.GitHubClienter) cli.ActionFunc {
	return func(c *cli.Context) error {
		identifier := c.Args().First()

		if identifier == "" {
			branch, err := git.CurrentBranch()
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

// Copies the current branch, or identifier PR's URL as a shareable link
func handlePRCopy(stdout, stderr io.Writer, ghCli gh.GitHubClienter) cli.ActionFunc {
	return func(c *cli.Context) error {
		if err := ensurePRContext(stdout, stderr, ghCli); err != nil {
			return err
		}

		identifier := c.Args().First()
		prStatus, err := ghCli.PRStatus(identifier)
		if err != nil {
			if identifier == "" {
				print.Error(stderr, "No pull request found for this branch")
				return cli.Exit("", 1)
			}
			print.Error(stderr, "No pull request found:", identifier)
			return cli.Exit("", 1)
		}
		url := strings.TrimSpace(prStatus.CurrentBranch.URL)
		title := strings.TrimSpace(prStatus.CurrentBranch.Title)

		if url == "" {
			print.Error(stderr, "No pull request URL available")
			return cli.Exit("", 1)
		}

		// Copy link to clipboard in multiple formats (markdown + HTML)
		if err := clipboard.CopyLink(title, url); err != nil {
			// Fallback: print to stdout
			print.Warning(stdout, print.WarningSym, "Could not access clipboard")
			fmt.Fprintln(stdout, url)
			return cli.Exit("", 0)
		}

		print.Success(stdout, print.Tick, "PR link copied to clipboard")

		return cli.Exit("", 0)
	}
}

func handlePRList(stdout, stderr io.Writer, ghCli gh.GitHubClienter) cli.ActionFunc {
	return func(c *cli.Context) error {
		if err := ensurePRContext(stdout, stderr, ghCli); err != nil {
			return err
		}

		var prs []gh.PullRequest
		err := spinner.With("Fetching pull requests", func() error {
			var fetchErr error
			prs, fetchErr = ghCli.ListPRs()
			return fetchErr
		}, spinner.WithWriter(stdout), spinner.WithFailureMessage("Failed to fetch pull requests"), spinner.WithSuccessMessage("Pull requests fetched"))
		if err != nil {
			return cli.Exit("", 1)
		}

		if len(prs) == 0 {
			print.Warning(stdout, print.Wrap("No open Pull Requests in this repository"))
			return nil
		}

		selected, action, err := promptPRList(stdout, prs)

		if err != nil {
			return err
		}

		switch action {
		case PROpen:
			if err := openPRInBrowser(stdout, ghCli, fmt.Sprint(selected.Number)); err != nil {
				return err
			}
		case PRCheckout:
			if err := git.Checkout(selected.HeadRefName); err != nil {
				return err
			}
		}

		return nil
	}
}

func bodyOrPRTemplate(c *cli.Context) (string, error) {
	body := c.String("body")
	if body == "" {
		body = git.GetPRTemplate()
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
	branch, err := git.CurrentBranch()
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
		return openPRInBrowser(stdout, ghCli, "")
	}

	return nil
}

const (
	PROpen     = "Open"
	PRCheckout = "Checkout"
)

func promptPRList(stdout io.Writer, prs []gh.PullRequest) (gh.PullRequest, string, error) {
	titles := make([]string, 0)
	for _, pr := range prs {
		titles = append(titles, pr.Title)
	}
	var selectedPRTitle string
	prompt := &survey.Select{
		Message: "Pull Request",
		Options: titles,
	}

	if err := survey.AskOne(prompt, &selectedPRTitle); err != nil {
		return gh.PullRequest{}, "", err
	}

	var action string
	actionPrompt := &survey.Select{
		Message: "Action",
		Options: []string{PROpen, PRCheckout},
	}
	if err := survey.AskOne(actionPrompt, &action); err != nil {
		return gh.PullRequest{}, "", err
	}

	idx := slices.IndexFunc(prs, func(pr gh.PullRequest) bool {
		return pr.Title == selectedPRTitle
	})

	return prs[idx], action, nil
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

func ensurePRContext(stdout, stderr io.Writer, ghCli gh.GitHubClienter) error {
	if !git.IsRepo() {
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

	return nil
}

func openPRInBrowser(stdout io.Writer, ghCli gh.GitHubClienter, identifier string) error {
	print.Info(stdout, print.Wrap(
		print.Arrow,
		"Opening pull request in browser...",
	))
	if err := ghCli.ViewPR(identifier); err != nil {
		print.Warning(stdout, print.Wrap(
			print.WarningSym,
			"Could not open PR in browser",
		))
		return cli.Exit("", 1)
	}
	return nil
}
