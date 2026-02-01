package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/thomasgormley/dev-cli-go/internal/linear"
	"github.com/urfave/cli/v2"
)

func getLinearClient() linear.Client {
	token := os.Getenv("LINEAR_API_KEY")
	return linear.NewClient(token)
}

func handleLinearCreate(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		title := c.String("title")
		description := c.String("description")
		teamID := os.Getenv("LINEAR_TEAM_ID")

		if teamID == "" {
			cli.Exit("LINEAR_TEAM_ID env var required", 1)
		}

		client := getLinearClient()
		issue, err := client.CreateIssue(c.Context, title, description, teamID)
		if err != nil {
			cli.Exit(fmt.Sprintf("failed to create issue: %v", err), 1)
		}

		output := map[string]string{
			"id":  string(issue.Identifier),
			"url": fmt.Sprintf("https://linear.app/issue/%s", string(issue.Identifier)),
		}
		data, _ := json.Marshal(output)
		fmt.Fprintln(stdout, string(data))
		return nil
	}
}

func handleLinearGet(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		if c.Args().Len() != 1 {
			cli.Exit("issue ID required", 1)
		}

		issueID := c.Args().First()
		client := getLinearClient()
		issue, err := client.GetIssue(c.Context, issueID)
		if err != nil {
			cli.Exit(fmt.Sprintf("failed to get issue: %v", err), 1)
		}

		output := map[string]string{
			"id":         string(issue.Identifier),
			"identifier": string(issue.Identifier),
			"title":      string(issue.Title),
		}
		data, _ := json.Marshal(output)
		fmt.Fprintln(stdout, string(data))
		return nil
	}
}

func handleLinearUpdate(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		if c.Args().Len() != 1 {
			cli.Exit("issue ID required", 1)
		}

		issueID := c.Args().First()
		title := c.String("title")
		description := c.String("description")

		if title == "" && description == "" {
			cli.Exit("at least --title or --description required", 1)
		}

		client := getLinearClient()
		issue, err := client.UpdateIssue(c.Context, issueID, title, description)
		if err != nil {
			cli.Exit(fmt.Sprintf("failed to update issue: %v", err), 1)
		}

		output := map[string]string{
			"id":         string(issue.Identifier),
			"identifier": string(issue.Identifier),
			"title":      string(issue.Title),
		}
		data, _ := json.Marshal(output)
		fmt.Fprintln(stdout, string(data))
		return nil
	}
}
