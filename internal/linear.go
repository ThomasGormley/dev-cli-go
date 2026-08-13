package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/urfave/cli/v2"

	linear "github.com/thomasgormley/dev-cli-go/internal/linear"
)

type linearCommandError struct {
	Code                string
	Message             string
	TeamCandidates      []linear.Team
	ProjectCandidates   []linear.Project
	MilestoneCandidates []linear.ProjectMilestone
	LabelCandidates     []linear.Label
	UserCandidates      []linear.User
	Err                 error
}

func (e *linearCommandError) Error() string {
	return e.Message
}

func (e *linearCommandError) Unwrap() error {
	return e.Err
}

type linearErrorOutput struct {
	Error struct {
		Code       string `json:"code"`
		Message    string `json:"message"`
		Candidates []any  `json:"candidates,omitempty"`
	} `json:"error"`
}

type linearTeamOutput struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type linearPageInfoOutput struct {
	HasNextPage bool   `json:"hasNextPage"`
	NextCursor  string `json:"nextCursor"`
}

type linearTeamListOutput struct {
	Items    []linearTeamOutput   `json:"items"`
	PageInfo linearPageInfoOutput `json:"pageInfo"`
}

type linearProjectOutput struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	SlugID string `json:"slugId"`
}

type linearProjectListOutput struct {
	Items    []linearProjectOutput `json:"items"`
	PageInfo linearPageInfoOutput  `json:"pageInfo"`
}

type linearLabelOutput struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	IsGroup bool              `json:"isGroup"`
	Team    *linearTeamOutput `json:"team"`
}

type linearLabelListOutput struct {
	Items    []linearLabelOutput  `json:"items"`
	PageInfo linearPageInfoOutput `json:"pageInfo"`
}

type linearProjectMilestoneOutput struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	TargetDate string `json:"targetDate"`
	Status     string `json:"status"`
}

type linearProjectMilestoneListOutput struct {
	Items    []linearProjectMilestoneOutput `json:"items"`
	PageInfo linearPageInfoOutput           `json:"pageInfo"`
}

type linearUserOutput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

type linearUserListOutput struct {
	Items    []linearUserOutput   `json:"items"`
	PageInfo linearPageInfoOutput `json:"pageInfo"`
}

type linearIssueOutput struct {
	ID               string                        `json:"id"`
	Identifier       string                        `json:"identifier"`
	URL              string                        `json:"url"`
	Title            string                        `json:"title"`
	Description      string                        `json:"description"`
	Team             linearTeamOutput              `json:"team"`
	Project          *linearProjectOutput          `json:"project"`
	ProjectMilestone *linearProjectMilestoneOutput `json:"projectMilestone"`
	Assignee         *linearUserOutput             `json:"assignee"`
	Priority         string                        `json:"priority"`
	Labels           []linearLabelOutput           `json:"labels"`
}

type linearDryRunOutput struct {
	DryRun    bool   `json:"dryRun"`
	Operation string `json:"operation"`
	Input     any    `json:"input"`
	Resolved  any    `json:"resolved,omitempty"`
}

type linearCreateResolvedOutput struct {
	Team             linearTeamOutput              `json:"team"`
	Project          *linearProjectOutput          `json:"project"`
	ProjectMilestone *linearProjectMilestoneOutput `json:"projectMilestone"`
	Labels           []linearLabelOutput           `json:"labels,omitempty"`
	Assignee         *linearUserOutput             `json:"assignee"`
	Priority         *string                       `json:"priority"`
}

type linearUpdateResolvedOutput struct {
	Project          *linearProjectOutput          `json:"project"`
	ProjectMilestone *linearProjectMilestoneOutput `json:"projectMilestone"`
	Labels           []linearLabelOutput           `json:"labels,omitempty"`
	Assignee         *linearUserOutput             `json:"assignee"`
	Priority         *string                       `json:"priority"`
}

type linearProjectChange struct {
	Selector string
	Clear    bool
}

type linearProjectMilestoneChange struct {
	Selector string
	Clear    bool
}

func (c linearProjectMilestoneChange) IsSet() bool {
	return c.Selector != "" || c.Clear
}

func (c linearProjectChange) IsSet() bool {
	return c.Selector != "" || c.Clear
}

type linearResolvedProjectChange struct {
	Project          *linearProjectOutput
	ProjectMilestone *linearProjectMilestoneOutput
	Changed          bool
}

type linearUpdateDryRunInput struct {
	ID     string
	Update linear.UpdateIssueRequest
}

const (
	linearPriorityNone = iota
	linearPriorityUrgent
	linearPriorityHigh
	linearPriorityMedium
	linearPriorityLow
)

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
		&cli.StringFlag{Name: "team", Usage: "team key or exact name"},
		&cli.StringFlag{Name: "project", Usage: "project slug or exact name"},
		&cli.StringSliceFlag{Name: "label", Usage: "label exact name (repeatable)"},
		&cli.StringFlag{Name: "milestone", Usage: "project milestone exact name"},
		&cli.BoolFlag{Name: "dry-run", Usage: "validate and print the mutation input without creating an issue"},
	)
}

func linearTeamListFlags() []cli.Flag {
	return []cli.Flag{
		&cli.IntFlag{Name: "limit", Value: 50, Usage: "maximum number of teams to return"},
		&cli.StringFlag{Name: "cursor", Usage: "cursor from a previous team list response"},
	}
}

func linearProjectListFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "team", Usage: "team key or exact name"},
		&cli.IntFlag{Name: "limit", Value: 50, Usage: "maximum number of projects to return"},
		&cli.StringFlag{Name: "cursor", Usage: "cursor from a previous project list response"},
	}
}

func linearLabelListFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "team", Usage: "team key or exact name"},
		&cli.IntFlag{Name: "limit", Value: 50, Usage: "maximum number of labels to return"},
		&cli.StringFlag{Name: "cursor", Usage: "cursor from a previous label list response"},
	}
}

func linearProjectMilestoneListFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "team", Usage: "team key or exact name"},
		&cli.StringFlag{Name: "project", Usage: "project slug or exact name"},
		&cli.IntFlag{Name: "limit", Value: 50, Usage: "maximum number of project milestones to return"},
		&cli.StringFlag{Name: "cursor", Usage: "cursor from a previous project milestone list response"},
	}
}

func linearUserListFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "team", Usage: "team key or exact name"},
		&cli.IntFlag{Name: "limit", Value: 50, Usage: "maximum number of active team members to return"},
		&cli.StringFlag{Name: "cursor", Usage: "cursor from a previous user list response"},
	}
}

func linearUpdateFlags() []cli.Flag {
	return append(linearMutationFlags(),
		&cli.BoolFlag{Name: "clear-description", Usage: "clear the issue description"},
		&cli.StringFlag{Name: "project", Usage: "project slug or exact name"},
		&cli.BoolFlag{Name: "clear-project", Usage: "remove the issue from its project"},
		&cli.BoolFlag{Name: "clear-assignee", Usage: "remove the issue assignee"},
		&cli.StringSliceFlag{Name: "add-label", Usage: "label exact name to add (repeatable)"},
		&cli.StringSliceFlag{Name: "remove-label", Usage: "label exact name to remove (repeatable)"},
		&cli.BoolFlag{Name: "clear-labels", Usage: "remove all issue labels"},
		&cli.StringFlag{Name: "milestone", Usage: "project milestone exact name"},
		&cli.BoolFlag{Name: "clear-milestone", Usage: "remove the issue from its project milestone"},
		&cli.BoolFlag{Name: "dry-run", Usage: "validate and print the mutation input without updating the issue"},
	)
}

func linearMutationFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "title", Usage: "title of the issue"},
		&cli.StringFlag{Name: "description", Usage: "description of the issue"},
		&cli.StringFlag{Name: "description-file", Usage: "read the description from a file or stdin (-)"},
		&cli.StringFlag{Name: "assignee", Usage: "assignee email, exact name, or me"},
		&cli.StringFlag{Name: "priority", Usage: "priority name: none, urgent, high, medium, or low"},
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
		if c.IsSet("milestone") && !c.IsSet("project") {
			return exitLinearError(stderr, invalidLinearArguments("--milestone requires --project"))
		}
		if err := requireLinearAPIKey(); err != nil {
			return exitLinearError(stderr, err)
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
		resolvedTeam, err := newLinearTeamOutput(team)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid resolved team", err))
		}

		resolvedProject, hasProject, err := resolveLinearProject(
			c.Context,
			client,
			resolvedTeam.ID,
			c.String("project"),
			c.IsSet("project"),
		)
		if err != nil {
			return exitLinearError(stderr, err)
		}
		labels, err := resolveLinearLabels(c.Context, client, resolvedTeam.ID, c.StringSlice("label"))
		if err != nil {
			return exitLinearError(stderr, err)
		}
		labelOutputs, err := newLinearLabelOutputs(labels)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid resolved label", err))
		}
		assignee, err := resolveLinearAssignee(c, client, resolvedTeam.ID)
		if err != nil {
			return exitLinearError(stderr, err)
		}
		priority, hasPriority, err := linearFlagPriority(c)
		if err != nil {
			return exitLinearError(stderr, err)
		}
		input := linear.IssueCreateInput{
			Title:       title,
			Description: description,
			TeamID:      resolvedTeam.ID,
			LabelIDs:    linearLabelIDs(labels),
		}
		if assignee != nil {
			input.AssigneeID = assignee.ID
		}
		if hasPriority {
			input.Priority = priority
		}
		if hasProject {
			input.ProjectID = resolvedProject.ID
		}
		resolvedMilestone, hasMilestone, err := resolveLinearProjectMilestone(
			c.Context,
			client,
			resolvedProject.ID,
			c.String("milestone"),
			c.IsSet("milestone"),
			hasProject,
		)
		if err != nil {
			return exitLinearError(stderr, err)
		}
		if hasMilestone {
			input.ProjectMilestoneID = resolvedMilestone.ID
		}
		if c.Bool("dry-run") {
			return writeLinearJSON(stdout, linearDryRunOutput{
				DryRun:    true,
				Operation: "create",
				Input:     input,
				Resolved: linearCreateResolvedOutput{
					Team:             resolvedTeam,
					Project:          optionalLinearProject(resolvedProject, hasProject),
					ProjectMilestone: optionalLinearProjectMilestone(resolvedMilestone, hasMilestone),
					Labels:           labelOutputs,
					Assignee:         assignee,
					Priority:         linearPriorityNameFromValue(priority, hasPriority),
				},
			})
		}

		issue, err := client.CreateIssue(c.Context, input)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("failed to create issue", err))
		}
		output, err := newLinearIssueOutput(
			issue,
			resolvedTeam,
			optionalLinearProject(resolvedProject, hasProject),
			optionalLinearProjectMilestone(resolvedMilestone, hasMilestone),
		)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid created issue", err))
		}
		return writeLinearJSON(stdout, output)
	}
}

func handleLinearLabelList(stdout, stderr io.Writer) cli.ActionFunc {
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
		page, err := client.ListLabels(c.Context, teamOutput.ID, linear.LabelListRequest{
			Limit: limit, Cursor: c.String("cursor"),
		})
		if err != nil {
			return exitLinearError(stderr, linearAPIError("failed to list labels", err))
		}
		output, err := newLinearLabelListOutput(page)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid label list response", err))
		}
		return writeLinearJSON(stdout, output)
	}
}

func handleLinearTeamList(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		if err := requireLinearAPIKey(); err != nil {
			return exitLinearError(stderr, err)
		}
		limit := c.Int("limit")
		if limit < 1 {
			return exitLinearError(stderr, invalidLinearArguments("--limit must be greater than zero"))
		}
		page, err := newLinearClient().ListTeams(c.Context, linear.TeamListRequest{
			Limit:  limit,
			Cursor: c.String("cursor"),
		})
		if err != nil {
			return exitLinearError(stderr, linearAPIError("failed to list teams", err))
		}
		output, err := newLinearTeamListOutput(page)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid team list response", err))
		}
		return writeLinearJSON(stdout, output)
	}
}

func handleLinearProjectList(stdout, stderr io.Writer) cli.ActionFunc {
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
		page, err := client.ListProjects(c.Context, teamOutput.ID, linear.ProjectListRequest{
			Limit:  limit,
			Cursor: c.String("cursor"),
		})
		if err != nil {
			return exitLinearError(stderr, linearAPIError("failed to list projects", err))
		}
		output, err := newLinearProjectListOutput(page)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid project list response", err))
		}
		return writeLinearJSON(stdout, output)
	}
}

func handleLinearProjectMilestoneList(stdout, stderr io.Writer) cli.ActionFunc {
	return func(c *cli.Context) error {
		if err := requireLinearAPIKey(); err != nil {
			return exitLinearError(stderr, err)
		}
		limit := c.Int("limit")
		if limit < 1 {
			return exitLinearError(stderr, invalidLinearArguments("--limit must be greater than zero"))
		}
		projectSelector := c.String("project")
		if !c.IsSet("project") || projectSelector == "" {
			return exitLinearError(stderr, invalidLinearArguments("--project required"))
		}
		teamSelector, err := linearTeamSelector(c)
		if err != nil {
			return exitLinearError(stderr, err)
		}
		client := newLinearClient()
		team, found, err := client.FindTeam(c.Context, teamSelector)
		if err != nil {
			return exitLinearError(stderr, linearTeamResolutionError(err))
		}
		if !found {
			return exitLinearError(stderr, linearTeamNotFound(teamSelector))
		}
		teamOutput, err := newLinearTeamOutput(team)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid resolved team", err))
		}
		project, _, err := resolveLinearProject(c.Context, client, teamOutput.ID, projectSelector, true)
		if err != nil {
			return exitLinearError(stderr, err)
		}
		page, err := client.ListProjectMilestones(c.Context, project.ID, linear.ProjectMilestoneListRequest{
			Limit: limit, Cursor: c.String("cursor"),
		})
		if err != nil {
			return exitLinearError(stderr, linearAPIError("failed to list project milestones", err))
		}
		output, err := newLinearProjectMilestoneListOutput(page)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid project milestone list response", err))
		}
		return writeLinearJSON(stdout, output)
	}
}

func handleLinearUserList(stdout, stderr io.Writer) cli.ActionFunc {
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
		page, err := client.ListUsers(c.Context, teamOutput.ID, linear.UserListRequest{
			Limit: limit, Cursor: c.String("cursor"),
		})
		if err != nil {
			return exitLinearError(stderr, linearAPIError("failed to list users", err))
		}
		output, err := newLinearUserListOutput(page)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid user list response", err))
		}
		return writeLinearJSON(stdout, output)
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

		issue, found, err := newLinearClient().FindIssue(c.Context, issueID)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("failed to find issue", err))
		}
		if !found {
			return exitLinearError(stderr, linearIssueNotFound(issueID))
		}
		team, err := newLinearTeamOutput(issue.Team)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid issue response", err))
		}
		output, err := newLinearIssueOutput(issue, team, nil, nil)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid issue response", err))
		}
		return writeLinearJSON(stdout, output)
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
		projectChange, err := linearProjectUpdate(c)
		if err != nil {
			return exitLinearError(stderr, err)
		}
		milestoneChange, err := linearProjectMilestoneUpdate(c)
		if err != nil {
			return exitLinearError(stderr, err)
		}
		if projectChange.Clear && milestoneChange.Selector != "" {
			return exitLinearError(stderr, invalidLinearArguments("--clear-project cannot be used with --milestone"))
		}
		if !hasLinearUpdate(input) && !projectChange.IsSet() && !milestoneChange.IsSet() && !c.IsSet("assignee") {
			return exitLinearError(stderr, invalidLinearArguments(
				"at least one issue field, project change, milestone change, or label change required",
			))
		}
		if err := requireLinearAPIKey(); err != nil {
			return exitLinearError(stderr, err)
		}
		client := newLinearClient()
		input, resolvedLabels, err := resolveLinearUpdateLabels(c.Context, client, issueID, input)
		if err != nil {
			return exitLinearError(stderr, err)
		}

		input, resolvedProject, err := applyLinearProjectMilestoneChanges(
			c.Context,
			client,
			issueID,
			input,
			projectChange,
			milestoneChange,
		)
		if err != nil {
			return exitLinearError(stderr, err)
		}
		input, resolvedAssignee, err := resolveLinearUpdateAssignee(c, client, issueID, input)
		if err != nil {
			return exitLinearError(stderr, err)
		}

		if c.Bool("dry-run") {
			return writeLinearJSON(
				stdout,
				newLinearUpdateDryRunOutput(
					issueID,
					input,
					resolvedProject,
					resolvedLabels,
					resolvedAssignee,
					c.IsSet("assignee") || input.ClearAssignee,
					linearPriorityNameFromValue(input.Priority, input.SetPriority),
				),
			)
		}
		issue, err := client.UpdateIssue(c.Context, issueID, input)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("failed to update issue", err))
		}
		team, err := newLinearTeamOutput(issue.Team)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid issue response", err))
		}
		output, err := newLinearIssueOutput(issue, team, nil, nil)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid issue response", err))
		}
		return writeLinearJSON(stdout, output)
	}
}

func resolveLinearUpdateLabels(
	ctx context.Context,
	client linear.Clienter,
	issueID string,
	input linear.UpdateIssueRequest,
) (linear.UpdateIssueRequest, []linearLabelOutput, error) {
	if len(input.AddedLabelIDs) == 0 && len(input.RemovedLabelIDs) == 0 {
		return input, nil, nil
	}
	issue, found, err := client.FindIssue(ctx, issueID)
	if err != nil {
		return linear.UpdateIssueRequest{}, nil, linearAPIError("failed to find issue for label resolution", err)
	}
	if !found {
		return linear.UpdateIssueRequest{}, nil, linearIssueNotFound(issueID)
	}
	team, err := newLinearTeamOutput(issue.Team)
	if err != nil {
		return linear.UpdateIssueRequest{}, nil, linearAPIError("invalid issue team", err)
	}
	addedLabels, err := resolveLinearLabels(ctx, client, team.ID, input.AddedLabelIDs)
	if err != nil {
		return linear.UpdateIssueRequest{}, nil, err
	}
	removedLabels, err := resolveLinearLabels(ctx, client, team.ID, input.RemovedLabelIDs)
	if err != nil {
		return linear.UpdateIssueRequest{}, nil, err
	}
	if err := ensureDistinctLabelChanges(addedLabels, removedLabels); err != nil {
		return linear.UpdateIssueRequest{}, nil, err
	}
	labelOutputs, err := newLinearLabelOutputs(append(addedLabels, removedLabels...))
	if err != nil {
		return linear.UpdateIssueRequest{}, nil, linearAPIError("invalid resolved label", err)
	}
	input.AddedLabelIDs = linearLabelIDs(addedLabels)
	input.RemovedLabelIDs = linearLabelIDs(removedLabels)
	return input, labelOutputs, nil
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
	input := linear.UpdateIssueRequest{
		ClearDescription: c.Bool("clear-description"),
		AddedLabelIDs:    c.StringSlice("add-label"),
		RemovedLabelIDs:  c.StringSlice("remove-label"),
		ClearLabels:      c.Bool("clear-labels"),
		ClearAssignee:    c.Bool("clear-assignee"),
	}
	if input.ClearAssignee && c.IsSet("assignee") {
		return linear.UpdateIssueRequest{}, invalidLinearArguments("--assignee and --clear-assignee cannot be used together")
	}
	if input.ClearLabels && (len(input.AddedLabelIDs) > 0 || len(input.RemovedLabelIDs) > 0) {
		return linear.UpdateIssueRequest{}, invalidLinearArguments(
			"--clear-labels cannot be used with --add-label or --remove-label",
		)
	}
	if c.IsSet("title") {
		title := c.String("title")
		if title == "" {
			return linear.UpdateIssueRequest{}, invalidLinearArguments("--title cannot be empty")
		}
		input.Title = title
		input.SetTitle = true
	}
	if c.IsSet("description") || c.IsSet("description-file") {
		input.Description = description
		input.SetDescription = true
	}
	priority, hasPriority, err := linearFlagPriority(c)
	if err != nil {
		return linear.UpdateIssueRequest{}, err
	}
	input.Priority = priority
	input.SetPriority = hasPriority
	return input, nil
}

func hasLinearUpdate(input linear.UpdateIssueRequest) bool {
	return input.SetTitle || input.SetDescription || input.ClearDescription ||
		input.SetProject || input.ClearProject || input.SetProjectMilestone || input.ClearProjectMilestone ||
		len(input.AddedLabelIDs) > 0 || len(input.RemovedLabelIDs) > 0 || input.ClearLabels ||
		input.SetAssignee || input.ClearAssignee || input.SetPriority
}

func linearProjectUpdate(c *cli.Context) (linearProjectChange, error) {
	hasProject := c.IsSet("project")
	clearProject := c.Bool("clear-project")
	if hasProject && clearProject {
		return linearProjectChange{}, invalidLinearArguments("--project and --clear-project cannot be used together")
	}
	if !hasProject {
		return linearProjectChange{Clear: clearProject}, nil
	}
	selector := c.String("project")
	if selector == "" {
		return linearProjectChange{}, invalidLinearArguments("--project cannot be empty")
	}
	return linearProjectChange{Selector: selector}, nil
}

func linearProjectMilestoneUpdate(c *cli.Context) (linearProjectMilestoneChange, error) {
	hasMilestone := c.IsSet("milestone")
	clearMilestone := c.Bool("clear-milestone")
	if hasMilestone && clearMilestone {
		return linearProjectMilestoneChange{}, invalidLinearArguments(
			"--milestone and --clear-milestone cannot be used together",
		)
	}
	if !hasMilestone {
		return linearProjectMilestoneChange{Clear: clearMilestone}, nil
	}
	selector := c.String("milestone")
	if selector == "" {
		return linearProjectMilestoneChange{}, invalidLinearArguments("--milestone cannot be empty")
	}
	return linearProjectMilestoneChange{Selector: selector}, nil
}

func applyLinearProjectMilestoneChanges(
	ctx context.Context,
	client linear.Clienter,
	issueID string,
	input linear.UpdateIssueRequest,
	projectChange linearProjectChange,
	milestoneChange linearProjectMilestoneChange,
) (linear.UpdateIssueRequest, linearResolvedProjectChange, error) {
	if !projectChange.IsSet() && !milestoneChange.IsSet() {
		return input, linearResolvedProjectChange{}, nil
	}
	if projectChange.Clear {
		input.ClearProject = true
		input.ClearProjectMilestone = true
		return input, linearResolvedProjectChange{Changed: true}, nil
	}
	if milestoneChange.Clear && !projectChange.IsSet() {
		input.ClearProjectMilestone = true
		return input, linearResolvedProjectChange{Changed: true}, nil
	}
	issue, project, err := resolveLinearUpdateProject(ctx, client, issueID, projectChange)
	if err != nil {
		return linear.UpdateIssueRequest{}, linearResolvedProjectChange{}, err
	}
	if milestoneChange.Selector != "" && project.ID == "" {
		return linear.UpdateIssueRequest{}, linearResolvedProjectChange{},
			invalidLinearArguments("--milestone requires --project or an issue project")
	}
	if projectChange.Selector != "" && issue.ProjectMilestone.ID != nil && !milestoneChange.IsSet() &&
		!projectMatchesLinearIssueProject(issue.Project, project.ID) {
		return linear.UpdateIssueRequest{}, linearResolvedProjectChange{}, invalidLinearArguments(
			"changing --project requires --milestone or --clear-milestone when the issue has a milestone",
		)
	}
	input, resolvedMilestone, hasMilestone, err := applyLinearMilestoneChange(ctx, client, input, project, milestoneChange)
	if err != nil {
		return linear.UpdateIssueRequest{}, linearResolvedProjectChange{}, err
	}
	if projectChange.Selector == "" {
		return input, linearResolvedProjectChange{
			Project:          optionalLinearProject(project, project.ID != ""),
			ProjectMilestone: optionalLinearProjectMilestone(resolvedMilestone, hasMilestone),
			Changed:          true,
		}, nil
	}

	input.ProjectID = project.ID
	input.SetProject = true
	return input, linearResolvedProjectChange{
		Project:          &project,
		ProjectMilestone: optionalLinearProjectMilestone(resolvedMilestone, hasMilestone),
		Changed:          true,
	}, nil
}

func resolveLinearUpdateProject(
	ctx context.Context,
	client linear.Clienter,
	issueID string,
	change linearProjectChange,
) (linear.Issue, linearProjectOutput, error) {
	issue, found, err := client.FindIssue(ctx, issueID)
	if err != nil {
		return linear.Issue{}, linearProjectOutput{}, linearAPIError("failed to find issue for project resolution", err)
	}
	if !found {
		return linear.Issue{}, linearProjectOutput{}, linearIssueNotFound(issueID)
	}
	if change.Selector == "" {
		if issue.Project.ID == nil {
			return issue, linearProjectOutput{}, nil
		}
		project, err := newLinearProjectOutput(issue.Project)
		if err != nil {
			return linear.Issue{}, linearProjectOutput{}, linearAPIError("invalid issue response", err)
		}
		return issue, project, nil
	}
	team, err := newLinearTeamOutput(issue.Team)
	if err != nil {
		return linear.Issue{}, linearProjectOutput{}, linearAPIError("invalid issue response", err)
	}
	project, _, err := resolveLinearProject(ctx, client, team.ID, change.Selector, true)
	return issue, project, err
}

func applyLinearMilestoneChange(
	ctx context.Context,
	client linear.Clienter,
	input linear.UpdateIssueRequest,
	project linearProjectOutput,
	change linearProjectMilestoneChange,
) (linear.UpdateIssueRequest, linearProjectMilestoneOutput, bool, error) {
	if change.Clear {
		input.ClearProjectMilestone = true
		return input, linearProjectMilestoneOutput{}, false, nil
	}
	if change.Selector == "" {
		return input, linearProjectMilestoneOutput{}, false, nil
	}
	milestone, found, err := resolveLinearProjectMilestone(ctx, client, project.ID, change.Selector, true, true)
	if err != nil {
		return linear.UpdateIssueRequest{}, linearProjectMilestoneOutput{}, false, err
	}
	input.ProjectMilestoneID = milestone.ID
	input.SetProjectMilestone = true
	return input, milestone, found, nil
}

func newLinearUpdateDryRunOutput(
	issueID string,
	input linear.UpdateIssueRequest,
	projectChange linearResolvedProjectChange,
	labels []linearLabelOutput,
	assignee *linearUserOutput,
	assigneeChanged bool,
	priority *string,
) linearDryRunOutput {
	output := linearDryRunOutput{
		DryRun:    true,
		Operation: "update",
		Input:     linearUpdateDryRunInput{ID: issueID, Update: input},
	}
	if projectChange.Changed || len(labels) > 0 || assigneeChanged || priority != nil {
		output.Resolved = linearUpdateResolvedOutput{
			Project:          projectChange.Project,
			ProjectMilestone: projectChange.ProjectMilestone,
			Labels:           labels,
			Assignee:         assignee,
			Priority:         priority,
		}
	}
	return output
}

func resolveLinearProject(
	ctx context.Context,
	client linear.Clienter,
	teamID string,
	selector string,
	hasProject bool,
) (linearProjectOutput, bool, error) {
	if !hasProject {
		return linearProjectOutput{}, false, nil
	}
	if selector == "" {
		return linearProjectOutput{}, false, invalidLinearArguments("--project cannot be empty")
	}
	project, found, err := client.FindProject(ctx, teamID, selector)
	if err != nil {
		return linearProjectOutput{}, false, linearProjectResolutionError(err)
	}
	if !found {
		return linearProjectOutput{}, false, linearProjectNotFound(selector)
	}
	output, err := newLinearProjectOutput(project)
	if err != nil {
		return linearProjectOutput{}, false, linearAPIError("invalid resolved project", err)
	}
	return output, true, nil
}

func resolveLinearLabels(
	ctx context.Context,
	client linear.Clienter,
	teamID string,
	selectors []string,
) ([]linear.Label, error) {
	labels := make([]linear.Label, 0, len(selectors))
	for _, selector := range selectors {
		if selector == "" {
			return nil, invalidLinearArguments("label selectors cannot be empty")
		}
		label, found, err := client.FindLabel(ctx, teamID, selector)
		if err != nil {
			return nil, linearLabelResolutionError(err)
		}
		if !found {
			return nil, linearLabelNotFound(selector)
		}
		if !containsLinearLabel(labels, label) {
			labels = append(labels, label)
		}
	}
	return labels, nil
}

func resolveLinearAssignee(c *cli.Context, client linear.Clienter, teamID string) (*linearUserOutput, error) {
	if !c.IsSet("assignee") {
		return nil, nil
	}
	selector := c.String("assignee")
	if selector == "" {
		return nil, invalidLinearArguments("--assignee cannot be empty")
	}
	user, found, err := client.FindAssignee(c.Context, teamID, selector)
	if err != nil {
		return nil, linearUserResolutionError(err)
	}
	if !found {
		return nil, linearUserNotFound(selector)
	}
	output, err := newLinearUserOutput(user)
	if err != nil {
		return nil, linearAPIError("invalid resolved assignee", err)
	}
	return &output, nil
}

func resolveLinearUpdateAssignee(
	c *cli.Context,
	client linear.Clienter,
	issueID string,
	input linear.UpdateIssueRequest,
) (linear.UpdateIssueRequest, *linearUserOutput, error) {
	if !c.IsSet("assignee") {
		return input, nil, nil
	}
	issue, found, err := client.FindIssue(c.Context, issueID)
	if err != nil {
		return linear.UpdateIssueRequest{}, nil, linearAPIError("failed to find issue for assignee resolution", err)
	}
	if !found {
		return linear.UpdateIssueRequest{}, nil, linearIssueNotFound(issueID)
	}
	team, err := newLinearTeamOutput(issue.Team)
	if err != nil {
		return linear.UpdateIssueRequest{}, nil, linearAPIError("invalid issue team", err)
	}
	resolvedAssignee, err := resolveLinearAssignee(c, client, team.ID)
	if err != nil {
		return linear.UpdateIssueRequest{}, nil, err
	}
	input.AssigneeID = resolvedAssignee.ID
	input.SetAssignee = true
	return input, resolvedAssignee, nil
}

func linearFlagPriority(c *cli.Context) (int, bool, error) {
	if !c.IsSet("priority") {
		return 0, false, nil
	}
	priority, err := linearPriority(c.String("priority"))
	if err != nil {
		return 0, false, err
	}
	return priority, true, nil
}

func linearPriority(value string) (int, error) {
	switch strings.ToLower(value) {
	case "none":
		return linearPriorityNone, nil
	case "urgent":
		return linearPriorityUrgent, nil
	case "high":
		return linearPriorityHigh, nil
	case "medium":
		return linearPriorityMedium, nil
	case "low":
		return linearPriorityLow, nil
	default:
		return linearPriorityNone, invalidLinearArguments("--priority must be none, urgent, high, medium, or low")
	}
}

func linearPriorityName(value int) string {
	switch value {
	case linearPriorityNone:
		return "none"
	case linearPriorityUrgent:
		return "urgent"
	case linearPriorityHigh:
		return "high"
	case linearPriorityMedium:
		return "medium"
	case linearPriorityLow:
		return "low"
	default:
		return "none"
	}
}

func linearPriorityNameFromValue(priority int, ok bool) *string {
	if !ok {
		return nil
	}
	name := linearPriorityName(priority)
	return &name
}

func ensureDistinctLabelChanges(addedLabels, removedLabels []linear.Label) error {
	for _, addedLabel := range addedLabels {
		if containsLinearLabel(removedLabels, addedLabel) {
			return invalidLinearArguments("the same label cannot be added and removed in one command")
		}
	}
	return nil
}

func containsLinearLabel(labels []linear.Label, target linear.Label) bool {
	targetID, ok := linearGraphQLIDString(target.ID)
	if !ok {
		return false
	}
	for _, label := range labels {
		id, ok := linearGraphQLIDString(label.ID)
		if ok && id == targetID {
			return true
		}
	}
	return false
}

func linearLabelIDs(labels []linear.Label) []string {
	if len(labels) == 0 {
		return nil
	}
	ids := make([]string, 0, len(labels))
	for _, label := range labels {
		id, ok := linearGraphQLIDString(label.ID)
		if !ok {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func resolveLinearProjectMilestone(
	ctx context.Context,
	client linear.Clienter,
	projectID string,
	selector string,
	hasMilestone bool,
	hasProject bool,
) (linearProjectMilestoneOutput, bool, error) {
	if !hasMilestone {
		return linearProjectMilestoneOutput{}, false, nil
	}
	if !hasProject {
		return linearProjectMilestoneOutput{}, false, invalidLinearArguments("--milestone requires --project")
	}
	if selector == "" {
		return linearProjectMilestoneOutput{}, false, invalidLinearArguments("--milestone cannot be empty")
	}
	milestone, found, err := client.FindProjectMilestone(ctx, projectID, selector)
	if err != nil {
		return linearProjectMilestoneOutput{}, false, linearProjectMilestoneResolutionError(err)
	}
	if !found {
		return linearProjectMilestoneOutput{}, false, linearProjectMilestoneNotFound(selector)
	}
	output, err := newLinearProjectMilestoneOutput(milestone)
	if err != nil {
		return linearProjectMilestoneOutput{}, false, linearAPIError("invalid resolved project milestone", err)
	}
	return output, true, nil
}

func projectMatchesLinearIssueProject(project linear.Project, projectID string) bool {
	id, ok := linearGraphQLIDString(project.ID)
	return ok && id == projectID
}

func linearGraphQLIDString(value any) (string, bool) {
	id, ok := value.(string)
	return id, ok && id != ""
}

func linearIssueID(c *cli.Context) (string, error) {
	if c.Args().Len() != 1 || c.Args().First() == "" {
		return "", invalidLinearArguments("issue ID required")
	}
	return c.Args().First(), nil
}

func linearTeamSelector(c *cli.Context) (string, error) {
	if c.IsSet("team") {
		selector := c.String("team")
		if selector == "" {
			return "", invalidLinearArguments("--team cannot be empty")
		}
		return selector, nil
	}
	selector := os.Getenv("LINEAR_TEAM_ID")
	if selector == "" {
		return "", &linearCommandError{Code: "missing_configuration", Message: "--team or LINEAR_TEAM_ID required"}
	}
	return selector, nil
}

func linearTeamResolutionError(err error) error {
	return linearResolverError("failed to resolve team", "ambiguous_team", err)
}

func linearProjectResolutionError(err error) error {
	return linearResolverError("failed to resolve project", "ambiguous_project", err)
}

func linearLabelResolutionError(err error) error {
	return linearResolverError("failed to resolve label", "invalid_label", err)
}

func linearProjectMilestoneResolutionError(err error) error {
	return linearResolverError("failed to resolve project milestone", "ambiguous_milestone", err)
}

func linearUserResolutionError(err error) error {
	return linearResolverError("failed to resolve assignee", "ambiguous_assignee", err)
}

func linearResolverError(apiMessage string, code string, err error) error {
	var apiErr *linear.APIError
	if errors.As(err, &apiErr) {
		return linearAPIError(apiMessage, err)
	}
	return &linearCommandError{Code: code, Message: err.Error(), Err: err}
}

func linearTeamNotFound(selector string) error {
	return &linearCommandError{Code: "team_not_found", Message: fmt.Sprintf("no active team matches %q", selector)}
}

func linearProjectNotFound(selector string) error {
	return &linearCommandError{Code: "project_not_found", Message: fmt.Sprintf("no active project matches %q", selector)}
}

func linearLabelNotFound(selector string) error {
	return &linearCommandError{Code: "label_not_found", Message: fmt.Sprintf("no applicable label matches %q", selector)}
}

func linearIssueNotFound(selector string) error {
	return &linearCommandError{Code: "issue_not_found", Message: fmt.Sprintf("no issue matches %q", selector)}
}

func linearProjectMilestoneNotFound(selector string) error {
	return &linearCommandError{
		Code:    "milestone_not_found",
		Message: fmt.Sprintf("no project milestone matches %q", selector),
	}
}

func linearUserNotFound(selector string) error {
	return &linearCommandError{
		Code:    "assignee_not_found",
		Message: fmt.Sprintf("no active team member matches %q", selector),
	}
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
	output := linearErrorOutput{}
	output.Error.Code = commandErr.Code
	output.Error.Message = commandErr.Message
	candidates, candidateErr := newLinearCandidates(commandErr)
	if candidateErr != nil {
		return cli.Exit(fmt.Errorf("encode Linear error candidates: %w", candidateErr), 1)
	}
	output.Error.Candidates = candidates
	if writeErr := writeLinearJSON(stderr, output); writeErr != nil {
		return cli.Exit(fmt.Errorf("write Linear error response: %w", writeErr), 1)
	}
	return cli.Exit("", 1)
}

func newLinearCandidates(commandErr *linearCommandError) ([]any, error) {
	teamCandidates, err := newLinearTeamOutputs(commandErr.TeamCandidates)
	if err != nil {
		return nil, err
	}
	projectCandidates, err := newLinearProjectOutputs(commandErr.ProjectCandidates)
	if err != nil {
		return nil, err
	}
	labelCandidates, err := newLinearLabelOutputs(commandErr.LabelCandidates)
	if err != nil {
		return nil, err
	}
	milestoneCandidates, err := newLinearProjectMilestoneOutputs(commandErr.MilestoneCandidates)
	if err != nil {
		return nil, err
	}
	userCandidates, err := newLinearUserOutputs(commandErr.UserCandidates)
	if err != nil {
		return nil, err
	}
	candidates := make(
		[]any,
		0,
		len(teamCandidates)+len(projectCandidates)+len(labelCandidates)+len(milestoneCandidates)+len(userCandidates),
	)
	for _, candidate := range teamCandidates {
		candidates = append(candidates, candidate)
	}
	for _, candidate := range projectCandidates {
		candidates = append(candidates, candidate)
	}
	for _, candidate := range labelCandidates {
		candidates = append(candidates, candidate)
	}
	for _, candidate := range milestoneCandidates {
		candidates = append(candidates, candidate)
	}
	for _, candidate := range userCandidates {
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func writeLinearJSON(writer io.Writer, value any) error {
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		return fmt.Errorf("encode Linear response: %w", err)
	}
	return nil
}

func newLinearIssueOutput(
	issue linear.Issue,
	team linearTeamOutput,
	fallbackProject *linearProjectOutput,
	fallbackProjectMilestone *linearProjectMilestoneOutput,
) (linearIssueOutput, error) {
	id, ok := linearGraphQLIDString(issue.ID)
	if !ok {
		return linearIssueOutput{}, fmt.Errorf("issue ID is not a string: %T", issue.ID)
	}
	project := fallbackProject
	projectMilestone := fallbackProjectMilestone
	if issue.Project.ID != nil {
		output, err := newLinearProjectOutput(issue.Project)
		if err != nil {
			return linearIssueOutput{}, err
		}
		project = &output
	}
	if issue.ProjectMilestone.ID != nil {
		output, err := newLinearProjectMilestoneOutput(issue.ProjectMilestone)
		if err != nil {
			return linearIssueOutput{}, err
		}
		projectMilestone = &output
	}
	assignee, err := newOptionalLinearUserOutput(issue.Assignee)
	if err != nil {
		return linearIssueOutput{}, err
	}
	labels, err := newLinearLabelOutputs(issue.Labels.Nodes)
	if err != nil {
		return linearIssueOutput{}, err
	}
	return linearIssueOutput{
		ID:               id,
		Identifier:       string(issue.Identifier),
		URL:              string(issue.URL),
		Title:            string(issue.Title),
		Description:      string(issue.Description),
		Team:             team,
		Project:          project,
		ProjectMilestone: projectMilestone,
		Assignee:         assignee,
		Priority:         linearPriorityName(int(issue.Priority)),
		Labels:           labels,
	}, nil
}

func newLinearTeamListOutput(page linear.TeamPage) (linearTeamListOutput, error) {
	items, err := newLinearTeamOutputs(page.Items)
	if err != nil {
		return linearTeamListOutput{}, err
	}
	return linearTeamListOutput{
		Items: items,
		PageInfo: linearPageInfoOutput{
			HasNextPage: page.PageInfo.HasNextPage,
			NextCursor:  page.PageInfo.NextCursor,
		},
	}, nil
}

func newLinearLabelListOutput(page linear.LabelPage) (linearLabelListOutput, error) {
	items, err := newLinearLabelOutputs(page.Items)
	if err != nil {
		return linearLabelListOutput{}, err
	}
	return linearLabelListOutput{
		Items: items,
		PageInfo: linearPageInfoOutput{
			HasNextPage: page.PageInfo.HasNextPage,
			NextCursor:  page.PageInfo.NextCursor,
		},
	}, nil
}

func newLinearUserListOutput(page linear.UserPage) (linearUserListOutput, error) {
	items, err := newLinearUserOutputs(page.Items)
	if err != nil {
		return linearUserListOutput{}, err
	}
	return linearUserListOutput{
		Items:    items,
		PageInfo: linearPageInfoOutput{HasNextPage: page.PageInfo.HasNextPage, NextCursor: page.PageInfo.NextCursor},
	}, nil
}

func newLinearTeamOutputs(teams []linear.Team) ([]linearTeamOutput, error) {
	items := make([]linearTeamOutput, 0, len(teams))
	for _, team := range teams {
		output, err := newLinearTeamOutput(team)
		if err != nil {
			return nil, err
		}
		items = append(items, output)
	}
	return items, nil
}

func newLinearProjectListOutput(page linear.ProjectPage) (linearProjectListOutput, error) {
	items, err := newLinearProjectOutputs(page.Items)
	if err != nil {
		return linearProjectListOutput{}, err
	}
	return linearProjectListOutput{
		Items: items,
		PageInfo: linearPageInfoOutput{
			HasNextPage: page.PageInfo.HasNextPage,
			NextCursor:  page.PageInfo.NextCursor,
		},
	}, nil
}

func newLinearProjectMilestoneListOutput(
	page linear.ProjectMilestonePage,
) (linearProjectMilestoneListOutput, error) {
	items, err := newLinearProjectMilestoneOutputs(page.Items)
	if err != nil {
		return linearProjectMilestoneListOutput{}, err
	}
	return linearProjectMilestoneListOutput{
		Items: items,
		PageInfo: linearPageInfoOutput{
			HasNextPage: page.PageInfo.HasNextPage,
			NextCursor:  page.PageInfo.NextCursor,
		},
	}, nil
}

func newLinearProjectOutputs(projects []linear.Project) ([]linearProjectOutput, error) {
	items := make([]linearProjectOutput, 0, len(projects))
	for _, project := range projects {
		output, err := newLinearProjectOutput(project)
		if err != nil {
			return nil, err
		}
		items = append(items, output)
	}
	return items, nil
}

func newLinearLabelOutputs(labels []linear.Label) ([]linearLabelOutput, error) {
	items := make([]linearLabelOutput, 0, len(labels))
	for _, label := range labels {
		output, err := newLinearLabelOutput(label)
		if err != nil {
			return nil, err
		}
		items = append(items, output)
	}
	return items, nil
}

func newLinearProjectMilestoneOutputs(
	milestones []linear.ProjectMilestone,
) ([]linearProjectMilestoneOutput, error) {
	items := make([]linearProjectMilestoneOutput, 0, len(milestones))
	for _, milestone := range milestones {
		output, err := newLinearProjectMilestoneOutput(milestone)
		if err != nil {
			return nil, err
		}
		items = append(items, output)
	}
	return items, nil
}

func newOptionalLinearUserOutput(user linear.User) (*linearUserOutput, error) {
	if user.ID == nil {
		return nil, nil
	}
	output, err := newLinearUserOutput(user)
	if err != nil {
		return nil, err
	}
	return &output, nil
}

func newLinearUserOutputs(users []linear.User) ([]linearUserOutput, error) {
	items := make([]linearUserOutput, 0, len(users))
	for _, user := range users {
		output, err := newLinearUserOutput(user)
		if err != nil {
			return nil, err
		}
		items = append(items, output)
	}
	return items, nil
}

func newLinearUserOutput(user linear.User) (linearUserOutput, error) {
	id, ok := linearGraphQLIDString(user.ID)
	if !ok {
		return linearUserOutput{}, fmt.Errorf("user ID is not a string: %T", user.ID)
	}
	return linearUserOutput{
		ID: id, Name: string(user.Name), DisplayName: string(user.DisplayName), Email: string(user.Email),
	}, nil
}

func newLinearTeamOutput(team linear.Team) (linearTeamOutput, error) {
	id, ok := linearGraphQLIDString(team.ID)
	if !ok {
		return linearTeamOutput{}, fmt.Errorf("team ID is not a string: %T", team.ID)
	}
	return linearTeamOutput{ID: id, Key: string(team.Key), Name: string(team.Name)}, nil
}

func newLinearProjectOutput(project linear.Project) (linearProjectOutput, error) {
	id, ok := linearGraphQLIDString(project.ID)
	if !ok {
		return linearProjectOutput{}, fmt.Errorf("project ID is not a string: %T", project.ID)
	}
	return linearProjectOutput{ID: id, Name: string(project.Name), SlugID: string(project.SlugID)}, nil
}

func newLinearProjectMilestoneOutput(milestone linear.ProjectMilestone) (linearProjectMilestoneOutput, error) {
	id, ok := linearGraphQLIDString(milestone.ID)
	if !ok {
		return linearProjectMilestoneOutput{}, fmt.Errorf("project milestone ID is not a string: %T", milestone.ID)
	}
	return linearProjectMilestoneOutput{
		ID:         id,
		Name:       string(milestone.Name),
		TargetDate: string(milestone.TargetDate),
		Status:     string(milestone.Status),
	}, nil
}

func optionalLinearProject(project linearProjectOutput, ok bool) *linearProjectOutput {
	if !ok {
		return nil
	}
	return &project
}

func newLinearLabelOutput(label linear.Label) (linearLabelOutput, error) {
	id, ok := linearGraphQLIDString(label.ID)
	if !ok {
		return linearLabelOutput{}, fmt.Errorf("label ID is not a string: %T", label.ID)
	}
	output := linearLabelOutput{ID: id, Name: string(label.Name), IsGroup: bool(label.IsGroup)}
	if label.Team.ID == nil {
		return output, nil
	}
	team, err := newLinearTeamOutput(label.Team)
	if err != nil {
		return linearLabelOutput{}, fmt.Errorf("label team: %w", err)
	}
	output.Team = &team
	return output, nil
}

func optionalLinearProjectMilestone(
	milestone linearProjectMilestoneOutput,
	ok bool,
) *linearProjectMilestoneOutput {
	if !ok {
		return nil
	}
	return &milestone
}
