package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v2"

	linear "github.com/thomasgormley/dev-cli-go/internal/linear"
)

type linearCommandError struct {
	Code    string
	Message string
	Err     error
}

func (e *linearCommandError) Error() string {
	return e.Message
}

func (e *linearCommandError) Unwrap() error {
	return e.Err
}

type linearErrorOutput struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type linearIssueOutput struct {
	ID          string `json:"id"`
	Identifier  string `json:"identifier"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type linearDryRunOutput struct {
	DryRun    bool   `json:"dryRun"`
	Operation string `json:"operation"`
	Input     any    `json:"input"`
}

type linearUpdateDryRunInput struct {
	ID     string
	Update linear.UpdateIssueRequest
}

func (i linearUpdateDryRunInput) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(i.Update)
	if err != nil {
		return nil, fmt.Errorf("encode update input: %w", err)
	}
	var output map[string]json.RawMessage
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("decode update input: %w", err)
	}
	id, err := json.Marshal(i.ID)
	if err != nil {
		return nil, fmt.Errorf("encode issue ID: %w", err)
	}
	output["id"] = id
	return json.Marshal(output)
}

var newLinearClient = func() linear.Clienter {
	return linear.NewClient(os.Getenv("LINEAR_API_KEY"))
}

func linearCreateFlags() []cli.Flag {
	return append(linearMutationFlags(),
		&cli.BoolFlag{Name: "dry-run", Usage: "validate and print the mutation input without creating an issue"},
	)
}

func linearUpdateFlags() []cli.Flag {
	return append(linearMutationFlags(),
		&cli.BoolFlag{Name: "clear-description", Usage: "clear the issue description"},
		&cli.BoolFlag{Name: "dry-run", Usage: "validate and print the mutation input without updating the issue"},
	)
}

func linearMutationFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "title", Usage: "title of the issue"},
		&cli.StringFlag{Name: "description", Usage: "description of the issue"},
		&cli.StringFlag{Name: "description-file", Usage: "read the description from a file or stdin (-)"},
	}
}

func linearUsageError(stderr io.Writer) cli.OnUsageErrorFunc {
	return func(_ *cli.Context, err error, _ bool) error {
		return exitLinearError(stderr, invalidLinearArguments(err.Error()))
	}
}

func handleLinearCreate(stdout, stderr io.Writer, stdin io.Reader) cli.ActionFunc {
	return func(c *cli.Context) error {
		title := c.String("title")
		if title == "" {
			return exitLinearError(stderr, invalidLinearArguments("--title required"))
		}

		description, err := linearDescription(c, stdin, false)
		if err != nil {
			return exitLinearError(stderr, err)
		}
		teamID, err := linearTeamID()
		if err != nil {
			return exitLinearError(stderr, err)
		}
		if err := requireLinearAPIKey(); err != nil {
			return exitLinearError(stderr, err)
		}

		input := linear.IssueCreateInput{Title: title, Description: description, TeamID: teamID}
		if c.Bool("dry-run") {
			return writeLinearJSON(stdout, linearDryRunOutput{DryRun: true, Operation: "create", Input: input})
		}

		issue, err := newLinearClient().CreateIssue(c.Context, input)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("failed to create issue", err))
		}
		return writeLinearJSON(stdout, newLinearIssueOutput(issue))
	}
}

func handleLinearGet(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		issueID, err := linearIssueID(c)
		if err != nil {
			return exitLinearError(stderr, err)
		}
		if err := requireLinearAPIKey(); err != nil {
			return exitLinearError(stderr, err)
		}

		issue, err := newLinearClient().GetIssue(c.Context, issueID)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("failed to get issue", err))
		}
		return writeLinearJSON(stdout, newLinearIssueOutput(issue))
	}
}

func handleLinearUpdate(stdout, stderr io.Writer, stdin io.Reader) cli.ActionFunc {
	return func(c *cli.Context) error {
		issueID, err := linearIssueID(c)
		if err != nil {
			return exitLinearError(stderr, err)
		}
		input, err := linearUpdateInput(c, stdin)
		if err != nil {
			return exitLinearError(stderr, err)
		}
		if err := requireLinearAPIKey(); err != nil {
			return exitLinearError(stderr, err)
		}

		if c.Bool("dry-run") {
			return writeLinearJSON(stdout, linearDryRunOutput{
				DryRun:    true,
				Operation: "update",
				Input:     linearUpdateDryRunInput{ID: issueID, Update: input},
			})
		}

		issue, err := newLinearClient().UpdateIssue(c.Context, issueID, input)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("failed to update issue", err))
		}
		return writeLinearJSON(stdout, newLinearIssueOutput(issue))
	}
}

func linearDescription(c *cli.Context, stdin io.Reader, allowClear bool) (string, error) {
	hasDescription := c.IsSet("description")
	hasFile := c.IsSet("description-file")
	if hasDescription && hasFile {
		return "", invalidLinearArguments("--description and --description-file cannot be used together")
	}
	if allowClear && c.Bool("clear-description") && (hasDescription || hasFile) {
		return "", invalidLinearArguments("--clear-description cannot be used with a description value")
	}
	if !hasFile {
		return c.String("description"), nil
	}

	path := c.String("description-file")
	if path == "-" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", &linearCommandError{
				Code:    "invalid_arguments",
				Message: fmt.Sprintf("read description from stdin: %v", err),
				Err:     err,
			}
		}
		return string(data), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", &linearCommandError{
			Code:    "invalid_arguments",
			Message: fmt.Sprintf("read description file: %v", err),
			Err:     err,
		}
	}
	return string(data), nil
}

func linearUpdateInput(c *cli.Context, stdin io.Reader) (linear.UpdateIssueRequest, error) {
	description, err := linearDescription(c, stdin, true)
	if err != nil {
		return linear.UpdateIssueRequest{}, err
	}
	input := linear.UpdateIssueRequest{ClearDescription: c.Bool("clear-description")}
	if c.IsSet("title") {
		title := c.String("title")
		if title == "" {
			return linear.UpdateIssueRequest{}, invalidLinearArguments("--title cannot be empty")
		}
		input.Title = &title
	}
	if c.IsSet("description") || c.IsSet("description-file") {
		input.Description = &description
	}
	if input.Title == nil && input.Description == nil && !input.ClearDescription {
		return linear.UpdateIssueRequest{}, invalidLinearArguments(
			"at least --title, --description, --description-file, or --clear-description required",
		)
	}
	return input, nil
}

func linearIssueID(c *cli.Context) (string, error) {
	if c.Args().Len() != 1 || c.Args().First() == "" {
		return "", invalidLinearArguments("issue ID required")
	}
	return c.Args().First(), nil
}

func linearTeamID() (string, error) {
	teamID := os.Getenv("LINEAR_TEAM_ID")
	if teamID == "" {
		return "", &linearCommandError{Code: "missing_configuration", Message: "LINEAR_TEAM_ID env var required"}
	}
	return teamID, nil
}

func requireLinearAPIKey() error {
	if os.Getenv("LINEAR_API_KEY") == "" {
		return &linearCommandError{Code: "missing_configuration", Message: "LINEAR_API_KEY env var required"}
	}
	return nil
}

func invalidLinearArguments(message string) error {
	return &linearCommandError{Code: "invalid_arguments", Message: message}
}

func linearAPIError(message string, err error) error {
	return &linearCommandError{Code: "api_error", Message: fmt.Sprintf("%s: %v", message, err), Err: err}
}

func exitLinearError(stderr io.Writer, err error) error {
	commandErr, ok := err.(*linearCommandError)
	if !ok {
		commandErr = &linearCommandError{Code: "internal_error", Message: "Linear command failed", Err: err}
	}
	if writeErr := writeLinearJSON(stderr, linearErrorOutput{Error: struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{Code: commandErr.Code, Message: commandErr.Message}}); writeErr != nil {
		return cli.Exit(fmt.Errorf("write Linear error response: %w", writeErr), 1)
	}
	return cli.Exit("", 1)
}

func writeLinearJSON(writer io.Writer, value any) error {
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		return fmt.Errorf("encode Linear response: %w", err)
	}
	return nil
}

func newLinearIssueOutput(issue linear.Issue) linearIssueOutput {
	return linearIssueOutput{
		ID:          issue.ID.(string),
		Identifier:  string(issue.Identifier),
		URL:         string(issue.URL),
		Title:       string(issue.Title),
		Description: string(issue.Description),
	}
}
