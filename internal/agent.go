package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/urfave/cli/v2"
)

func handleAgentDispatch(client *http.Client) cli.ActionFunc {

	return func(c *cli.Context) error {
		urlArg := c.Args().First()
		if urlArg == "" {
			return errors.New("url is required")
		}

		reqBody := map[string]string{"url": urlArg}
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("encoding body: %w", err)
		}

		reqUrl := baseURL + "/api/agent/dispatch"
		req, err := http.NewRequest("POST", reqUrl, bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		rsp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("dispatching to agent: %w", err)
		}

		defer rsp.Body.Close()
		bytes, err := io.ReadAll(rsp.Body)
		if err != nil {
			return fmt.Errorf("reading body: %w", err)
		}
		log.Printf("[%d] %s", rsp.StatusCode, bytes)
		return nil
	}
}
