package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/thomasgormley/dev-cli-go/internal/serve"
	"github.com/urfave/cli/v2"
)

func serveBeforeFunc(client serve.Client) cli.BeforeFunc {
	return func(c *cli.Context) error {
		if !client.IsServerRunning(c.Context) {
			return errors.New("dev-cli server not running start with: dev serve")
		}
		return nil
	}
}

func handleAgentDispatch(client serve.Client) cli.ActionFunc {
	return func(c *cli.Context) error {
		urlArg := c.Args().First()
		if urlArg == "" {
			return errors.New("url is required")
		}

		_, err := client.DispatchAgent(c.Context, urlArg)
		if err != nil {
			return err
		}

		return nil
	}
}

func handleAgentAttach(client serve.Client) cli.ActionFunc {
	return func(c *cli.Context) error {
		urlArg := c.String("url")
		opencodeURL := urlArg

		if opencodeURL == "" {
			health, err := client.GetHealth(c.Context)
			if err != nil {
				return errors.New("failed to get health from server")
			}
			if !health.OpenCodeLive {
				return errors.New("OpenCode server not running")
			}
			opencodeURL = health.OpenCodeURL
		}

		fmt.Fprintf(c.App.Writer, "Opening %s in browser...\n", opencodeURL)

		browserCmd := exec.Command("open", opencodeURL)
		browserCmd.Stdout = os.Stdout
		browserCmd.Stderr = os.Stderr
		return browserCmd.Run()
	}
}
