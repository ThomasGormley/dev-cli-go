package cli

import (
	"io"

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
						Name:    "copy",
						Usage:   "Copy a pull request URL as a shareable link",
						Aliases: []string{"c"},
						Action:  handlePRCopy(stdout, stderr, ghClient),
					},
					{
						Name:    "list",
						Usage:   "List pull requests",
						Aliases: []string{"l"},
						Action:  handlePRList(stdout, stderr, ghClient),
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
						Usage:   "Open the diary, defaults to today's entry",
						Aliases: []string{"o"},
						Flags:   []cli.Flag{&cli.BoolFlag{Name: "repo-only", Aliases: []string{"ro"}}},
						Action:  handleDiaryOpen(stdout, stderr),
					},
					{
						Name:    "paste",
						Usage:   "Append clipboard contents to today's diary entry",
						Aliases: []string{"p"},
						Action:  handleDiaryPaste(stdout, stderr),
					},
					{
						Name:   "sync",
						Usage:  "Sync diary entries to remote",
						Action: handleDiarySync(stdout, stderr),
					},
				},
			},
			{
				Name:    "test",
				Usage:   "Testing utilities",
				Aliases: []string{"t"},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "all",
						Usage:   "runs all tests",
						Aliases: []string{"a"},
						Value:   false,
					},
					&cli.BoolFlag{
						Name:    "verbose",
						Usage:   "run tests with verbose output",
						Aliases: []string{"v"},
						Value:   false,
					},
					&cli.BoolFlag{
						Name:    "rerun",
						Usage:   "re-run the previously ran test command",
						Aliases: []string{"r"},
						Value:   false,
					},
				},
				Action: handleTest(stdout, stderr),
			},
		},
	}

	return app.Run(args)
}
