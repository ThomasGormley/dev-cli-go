package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/urfave/cli/v2"

	linear "github.com/thomasgormley/dev-cli-go/internal/linear"
)

type linearWorkflowStateOutput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type linearWorkflowStateListOutput struct {
	Items    []linearWorkflowStateOutput `json:"items"`
	PageInfo linearPageInfoOutput        `json:"pageInfo"`
}

func linearStateListFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "team", Usage: "team key or exact name"},
		&cli.IntFlag{Name: "limit", Value: 50, Usage: "maximum number of workflow states to return"},
		&cli.StringFlag{Name: "cursor", Usage: "cursor from a previous workflow state list response"},
	}
}

func handleLinearStateList(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		if err := requireLinearAPIKey(); err != nil {
			return exitLinearError(stderr, err)
		}
		limit := c.Int("limit")
		if limit < 1 {
			return exitLinearError(stderr, invalidLinearArguments("--limit must be greater than zero"))
		}
		selector, err := linearTeamSelector(c)
		if err != nil {
			return exitLinearError(stderr, err)
		}
		client := newLinearClient()
		team, found, err := client.FindTeam(c.Context, selector)
		if err != nil {
			return exitLinearError(stderr, linearTeamResolutionError(err))
		}
		if !found {
			return exitLinearError(stderr, linearTeamNotFound(selector))
		}
		teamOutput, err := newLinearTeamOutput(team)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid resolved team", err))
		}
		page, err := client.ListWorkflowStates(c.Context, teamOutput.ID, linear.WorkflowStateListRequest{
			Limit: limit, Cursor: c.String("cursor"),
		})
		if err != nil {
			return exitLinearError(stderr, linearAPIError("failed to list workflow states", err))
		}
		output, err := newLinearWorkflowStateListOutput(page)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid workflow state list response", err))
		}
		return writeLinearJSON(stdout, output)
	}
}

func linearStateSelector(c *cli.Context) (string, error) {
	if !c.IsSet("state") {
		return "", nil
	}
	selector := c.String("state")
	if selector == "" {
		return "", invalidLinearArguments("--state cannot be empty")
	}
	return selector, nil
}

func resolveLinearUpdateState(
	ctx context.Context,
	client linear.Clienter,
	issueID string,
	input linear.UpdateIssueRequest,
	selector string,
) (linear.UpdateIssueRequest, *linearWorkflowStateOutput, error) {
	if selector == "" {
		return input, nil, nil
	}
	issue, err := findLinearIssue(ctx, client, issueID)
	if err != nil {
		return linear.UpdateIssueRequest{}, nil, err
	}
	team, err := newLinearTeamOutput(issue.Team)
	if err != nil {
		return linear.UpdateIssueRequest{}, nil, linearAPIError("invalid issue team", err)
	}
	state, found, err := client.FindWorkflowState(ctx, team.ID, selector)
	if err != nil {
		return linear.UpdateIssueRequest{}, nil, linearResolverError(
			"failed to resolve workflow state", "ambiguous_state", err,
		)
	}
	if !found {
		return linear.UpdateIssueRequest{}, nil, &linearCommandError{
			Code:    "state_not_found",
			Message: fmt.Sprintf("no workflow state matches %q", selector),
		}
	}
	output, err := newLinearWorkflowStateOutput(state)
	if err != nil {
		return linear.UpdateIssueRequest{}, nil, linearAPIError("invalid resolved workflow state", err)
	}
	input.StateID = output.ID
	input.SetState = true
	return input, &output, nil
}

func newLinearWorkflowStateListOutput(page linear.WorkflowStatePage) (linearWorkflowStateListOutput, error) {
	items := make([]linearWorkflowStateOutput, 0, len(page.Items))
	for _, state := range page.Items {
		output, err := newLinearWorkflowStateOutput(state)
		if err != nil {
			return linearWorkflowStateListOutput{}, err
		}
		items = append(items, output)
	}
	return linearWorkflowStateListOutput{
		Items:    items,
		PageInfo: linearPageInfoOutput{HasNextPage: page.PageInfo.HasNextPage, NextCursor: page.PageInfo.NextCursor},
	}, nil
}

func newLinearWorkflowStateOutput(state linear.WorkflowState) (linearWorkflowStateOutput, error) {
	id, ok := linearGraphQLIDString(state.ID)
	if !ok {
		return linearWorkflowStateOutput{}, fmt.Errorf("workflow state ID is not a string: %T", state.ID)
	}
	return linearWorkflowStateOutput{ID: id, Name: string(state.Name), Type: string(state.Type)}, nil
}

func newOptionalLinearWorkflowStateOutput(state linear.WorkflowState) (*linearWorkflowStateOutput, error) {
	if state.ID == nil {
		return nil, nil
	}
	output, err := newLinearWorkflowStateOutput(state)
	if err != nil {
		return nil, err
	}
	return &output, nil
}
