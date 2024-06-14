package cli

import (
	"io"

	"github.com/urfave/cli/v2"
)

// type getEnvFunc func(string) string

func Run(
	args []string,
	stdout,
	stderr io.Writer,
	ghClient GitHubClienter,
	exitErrorHandler cli.ExitErrHandlerFunc,
) error {

	app := &cli.App{
		Name:                 "dev-cli",
		Usage:                "a simple dev cli",
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
								Value:   false,
							},
						},
					},
				},
			},
			{
				// Diary definition
				Name:  "diary",
				Usage: "For working with engineering diaries",
				Subcommands: []*cli.Command{
					{
						Name:   "new",
						Usage:  "Create a new diary entry",
						Action: handleDiaryNew(stdout, stderr),
					},
					{
						Name:   "open",
						Usage:  "Open today's diary entry",
						Action: handleDiaryOpen(stdout, stderr),
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
