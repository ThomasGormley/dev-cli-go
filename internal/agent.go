package cli

import (
	"errors"

	"github.com/thomasgormley/dev-cli-go/internal/serve"
	"github.com/urfave/cli/v2"
)

func handleAgentDispatch(client serve.Client) cli.ActionFunc {
	return func(c *cli.Context) error {

		if !client.IsServerRunning(c.Context) {
			return errors.New("dev-cli server not running start with: dev serve")
		}

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
