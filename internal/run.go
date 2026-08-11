package cli

import (
	"context"
	"io"
	"log"
	"os"
	"os/signal"

	"github.com/thomasgormley/dev-cli-go/internal/gh"
	"github.com/thomasgormley/dev-cli-go/internal/serve"
	"github.com/urfave/cli/v2"
)

const (
	defaultPort = "1967"
	defaultHost = "localhost"
	baseURL     = "http://" + defaultHost + ":" + defaultPort
)

func Run(
	args []string,
	stdout,
	stderr io.Writer,
	ghClient gh.GitHubClienter,
	exitErrorHandler cli.ExitErrHandlerFunc,
) error {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)
	go func() {
		select {
		case <-sigc:
			log.Print("Cleaning up. Press Ctrl-C again to exit immediately.")
			cancel()
		case <-ctx.Done():
		}
	}()

	serveClient := serve.NewClient(baseURL)
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
							&cli.BoolFlag{
								Name:    "force",
								Usage:   "skip prompts and use defaults",
								Aliases: []string{"f"},
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
						Action:  handlePRList(stdout, stderr, ghClient, serveClient),
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
			{
				Name:        "serve",
				Usage:       "Start the dev agent HTTP server",
				Description: "Starts an HTTP server that processes GitHub PR comments and dispatches AI agent tasks",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "host",
						Value: defaultHost,
					},
					&cli.StringFlag{
						Name:  "port",
						Value: defaultPort,
					},
					&cli.StringFlag{
						Name:     "provider",
						Usage:    "opencode provider ID",
						EnvVars:  []string{"DEV_AGENT_PROVIDER"},
						Required: true,
					},
					&cli.StringFlag{
						Name:     "model",
						Usage:    "opencode model ID",
						EnvVars:  []string{"DEV_AGENT_MODEL"},
						Required: true,
					},
					&cli.StringFlag{
						Name:     "github-token",
						Usage:    "GitHub API token",
						EnvVars:  []string{"DEV_GITHUB_TOKEN"},
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "start-opencode",
						Usage: "start OpenCode server",
						Value: true,
					},
					&cli.StringFlag{
						Name:  "opencode-host",
						Value: "localhost",
						Usage: "OpenCode server host",
					},
					&cli.StringFlag{
						Name:  "opencode-port",
						Value: "3366",
						Usage: "OpenCode server port",
					},
				},
				Action: handleServe(),
			},
			{
				Name:        "agent",
				Usage:       "Dev agent operations",
				Description: "Commands for interacting with the dev agent service",
				Before:      serveBeforeFunc(serveClient),
				Subcommands: []*cli.Command{
					{
						Name:        "dispatch",
						Usage:       "Dispatch a PR URL to the agent for processing",
						Description: "Sends a GitHub PR URL to the running dev agent server for AI processing",
						Args:        true,
						Action:      handleAgentDispatch(serveClient),
					},
					{
						Name:        "attach",
						Usage:       "Open the dev agent web UI",
						Description: "Opens the OpenCode web UI in the browser",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "url",
								Usage: "URL of the OpenCode server (auto-detected from serve if not provided)",
							},
						},
						Action: handleAgentAttach(serveClient),
					},
				},
			},
			{
				Name:        "linear",
				Usage:       "Linear issue management",
				Description: "Create, view, and update Linear issues",
				Subcommands: []*cli.Command{
					{
						Name:         "create",
						Usage:        "Create a new Linear issue",
						Flags:        linearCreateFlags(),
						OnUsageError: linearUsageError(stderr),
						Action:       handleLinearCreate(stdout, stderr, os.Stdin),
					},
					{
						Name:         "get",
						Usage:        "Get an issue by ID",
						ArgsUsage:    "<issue-id>",
						OnUsageError: linearUsageError(stderr),
						Action:       handleLinearGet(stdout, stderr),
					},
					{
						Name:         "update",
						Usage:        "Update an issue",
						ArgsUsage:    "<issue-id>",
						Flags:        linearUpdateFlags(),
						OnUsageError: linearUsageError(stderr),
						Action:       handleLinearUpdate(stdout, stderr, os.Stdin),
					},
					{
						Name:  "team",
						Usage: "Discover accessible Linear teams",
						Subcommands: []*cli.Command{
							{
								Name:         "list",
								Usage:        "List active accessible teams",
								Flags:        linearTeamListFlags(),
								OnUsageError: linearUsageError(stderr),
								Action:       handleLinearTeamList(stdout, stderr),
							},
						},
					},
					{
						Name:  "project",
						Usage: "Discover accessible Linear projects",
						Subcommands: []*cli.Command{
							{
								Name:         "list",
								Usage:        "List active accessible projects for a team",
								Flags:        linearProjectListFlags(),
								OnUsageError: linearUsageError(stderr),
								Action:       handleLinearProjectList(stdout, stderr),
							},
						},
					},
				},
			},
		},
	}

	return app.RunContext(ctx, args)
}
