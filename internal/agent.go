package cli

import (
	"errors"
	"fmt"

	"github.com/thomasgormley/dev-cli-go/internal/serve"
	"github.com/urfave/cli/v2"
)

func handleAgentDispatch(apiURL string) cli.ActionFunc {
	return func(c *cli.Context) error {
		client := serve.NewClient(apiURL)

		if !client.IsServerRunning(c.Context) {
			return fmt.Errorf("server not running at %s\nStart with: dev serve", apiURL)
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
