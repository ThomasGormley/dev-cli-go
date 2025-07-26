package cli

import (
	"io"
	"os"

	"github.com/google/go-github/v74/github"
	"github.com/thomasgormley/dev-cli-go/internal/gh"
	"github.com/urfave/cli/v2"
)

// type getEnvFunc func(string) string

func Run(
	args []string,
	stdout,
	stderr io.Writer,
	ghClient gh.GitHubClienter,
	exitErrorHandler cli.ExitErrHandlerFunc,
) error {

	ghHttpClient := github.NewClient(nil).WithAuthToken(os.Getenv("GH_TOKEN"))

	app := &cli.App{
		Name:                 "dev",
		HelpName:             "dev",
		Usage:                "Personal development CLI toolbox",
		ExitErrHandler:       exitErrorHandler,
		EnableBashCompletion: true,
		Commands: []*cli.Command{
			// PR definition
			{
				Name:  "pr",
				Usage: "Wrapper around gh cli",
				Subcommands: []*cli.Command{
					{
						Name:   "create",
						Usage:  "Create a new pull request",
						Action: handlePRCreate(stdout, stderr, ghClient),
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "title",
								Usage:   "title of the pull request",
								Aliases: []string{"t"},
							},
							&cli.StringFlag{
								Name:    "body",
								Usage:   "body of the pull request",
								Aliases: []string{"b"},
							},
							&cli.StringFlag{
								Name:    "base",
								Usage:   "base branch",
								Aliases: []string{"B"},
								EnvVars: []string{"TEAM_BRANCH"},
							},
							&cli.BoolFlag{
								Name:    "draft",
								Usage:   "mark the pull request as a draft",
								Aliases: []string{"d"},
								Value:   true,
							},
						},
					},
					{
						Name:    "view",
						Usage:   "View a pull request",
						Aliases: []string{"v"},
						Action:  handlePRView(stdout, stderr, ghClient),
					},
					{
						Name:    "merge",
						Usage:   "Merge a pull request",
						Aliases: []string{"m"},
						Action:  handlePRMerge(stdout, stderr, ghClient),
					},
					{
						Name:    "review",
						Usage:   "Review a pull request",
						Aliases: []string{"r"},
						Action:  handlePRReview(stdout, stderr, ghHttpClient),
					},
				},
			},
			{
				// Diary definition
				Name:    "diary",
				Usage:   "For working with engineering diaries",
				Aliases: []string{"d"},
				Subcommands: []*cli.Command{
					{
						Name:    "new",
						Usage:   "Create a new diary entry",
						Aliases: []string{"n"},
						Action:  handleDiaryNew(stdout, stderr),
					},
					{
						Name:    "open",
						Usage:   "Open today's diary entry",
						Aliases: []string{"o"},
						Action:  handleDiaryOpen(stdout, stderr),
					},
					{
						Name:   "sync",
						Usage:  "Sync diary entries to remote",
						Action: handleDiarySync(stdout, stderr),
					},
				},
			},
		},
	}

	return app.Run(args)
}
