package cli

import (
	"fmt"
	"io"

	"github.com/urfave/cli/v2"
)

// type getEnvFunc func(string) string

func Run(
	args []string,
	stdin io.Reader,
	stdout,
	stderr io.Writer,
	exitErrorHandler cli.ExitErrHandlerFunc,
) error {

	fmt.Printf("Running the cli with args: %v\n", args)
	stdout.Write([]byte("Running the cli\n"))
	app := &cli.App{
		Name:           "dev-cli",
		Usage:          "A simple dev cli",
		ExitErrHandler: exitErrorHandler,
		Commands: []*cli.Command{
			{
				Name:  "pr",
				Usage: "Wrapper around gh cli",
				Subcommands: []*cli.Command{
					{
						Name:   "create",
						Usage:  "Create a new pull request",
						Action: handlePRCreate(stdout, stderr),
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "title",
								Usage:   "Title of the pull request",
								Aliases: []string{"t"},
							},
							&cli.StringFlag{
								Name:    "body",
								Usage:   "Body of the pull request",
								Aliases: []string{"b"},
							},
							&cli.StringFlag{
								Name:    "base",
								Usage:   "Base branch",
								Aliases: []string{"B"},
							},
						},
					},
				},
			},
		},
	}

	return app.Run(args)
}

func writeError(stderr io.Writer, format string, a ...any) {
	fmt.Fprintf(stderr, format, a...)
}

func writeOut(stdout io.Writer, format string, a ...any) {
	fmt.Fprintf(stdout, format, a...)
}
