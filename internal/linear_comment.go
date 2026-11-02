package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/urfave/cli/v2"

	linear "github.com/thomasgormley/dev-cli-go/internal/linear"
)

type linearCommentOutput struct {
	ID    string               `json:"id"`
	URL   string               `json:"url"`
	Body  string               `json:"body"`
	Issue linearIssueRefOutput `json:"issue"`
}

type linearCommentResolvedOutput struct {
	Issue linearIssueRefOutput `json:"issue"`
}

func linearCommentFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "body", Usage: "markdown body of the comment"},
		&cli.StringFlag{Name: "body-file", Usage: "read the comment body from a file or stdin (-)"},
		&cli.BoolFlag{Name: "dry-run", Usage: "validate and print the mutation input without posting the comment"},
	}
}

func handleLinearComment(stdout, stderr io.Writer, stdin io.Reader) cli.ActionFunc {
	return func(c *cli.Context) error {
		issueID, err := linearIssueID(c)
		if err != nil {
			return exitLinearError(stderr, err)
		}
		body, err := linearCommentBody(c, stdin)
		if err != nil {
			return exitLinearError(stderr, err)
		}
		if err := requireLinearAPIKey(); err != nil {
			return exitLinearError(stderr, err)
		}
		client := newLinearClient()
		issue, err := resolveLinearIssueRef(c.Context, client, issueID)
		if err != nil {
			return exitLinearError(stderr, err)
		}
		input := linear.CommentCreateInput{IssueID: issue.ID, Body: body}
		if c.Bool("dry-run") {
			return writeLinearJSON(stdout, linearDryRunOutput{
				DryRun:    true,
				Operation: "comment",
				Input:     input,
				Resolved:  linearCommentResolvedOutput{Issue: issue},
			})
		}

		comment, err := client.CreateComment(c.Context, input)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("failed to create comment", err))
		}
		output, err := newLinearCommentOutput(comment)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid created comment", err))
		}
		return writeLinearJSON(stdout, output)
	}
}

func linearCommentBody(c *cli.Context, stdin io.Reader) (string, error) {
	if !c.IsSet("body") && !c.IsSet("body-file") {
		return "", invalidLinearArguments("--body or --body-file required")
	}
	body, err := linearTextFlag(c, stdin, "body")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(body) == "" {
		return "", invalidLinearArguments("comment body cannot be empty")
	}
	return body, nil
}

func newLinearCommentOutput(comment linear.Comment) (linearCommentOutput, error) {
	id, ok := linearGraphQLIDString(comment.ID)
	if !ok {
		return linearCommentOutput{}, fmt.Errorf("comment ID is not a string: %T", comment.ID)
	}
	issue, err := newLinearIssueRefOutput(comment.Issue)
	if err != nil {
		return linearCommentOutput{}, fmt.Errorf("comment issue: %w", err)
	}
	return linearCommentOutput{ID: id, URL: string(comment.URL), Body: string(comment.Body), Issue: issue}, nil
}
