package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v2"

	linear "github.com/thomasgormley/dev-cli-go/internal/linear"
)

type linearCommandError struct {
	Code              string
	Message           string
	TeamCandidates    []linear.Team
	ProjectCandidates []linear.Project
	Err               error
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

type linearIssueOutput struct {
	ID          string               `json:"id"`
	Identifier  string               `json:"identifier"`
	URL         string               `json:"url"`
	Title       string               `json:"title"`
	Description string               `json:"description"`
	Team        linearTeamOutput     `json:"team"`
	Project     *linearProjectOutput `json:"project"`
}

type linearDryRunOutput struct {
	DryRun    bool   `json:"dryRun"`
	Operation string `json:"operation"`
	Input     any    `json:"input"`
	Resolved  any    `json:"resolved,omitempty"`
}

type linearCreateResolvedOutput struct {
	Team    linearTeamOutput     `json:"team"`
	Project *linearProjectOutput `json:"project"`
}

type linearUpdateResolvedOutput struct {
	Project *linearProjectOutput `json:"project"`
}

type linearProjectChange struct {
	Selector string
	Clear    bool
}

func (c linearProjectChange) IsSet() bool {
	return c.Selector != "" || c.Clear
}

type linearResolvedProjectChange struct {
	Project *linearProjectOutput
	Changed bool
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
		&cli.StringFlag{Name: "team", Usage: "team UUID, key, or exact name"},
		&cli.StringFlag{Name: "project", Usage: "project UUID, slug, or exact name"},
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
		&cli.StringFlag{Name: "team", Usage: "team UUID, key, or exact name"},
		&cli.IntFlag{Name: "limit", Value: 50, Usage: "maximum number of projects to return"},
		&cli.StringFlag{Name: "cursor", Usage: "cursor from a previous project list response"},
	}
}

func linearUpdateFlags() []cli.Flag {
	return append(linearMutationFlags(),
		&cli.BoolFlag{Name: "clear-description", Usage: "clear the issue description"},
		&cli.StringFlag{Name: "project", Usage: "project UUID, slug, or exact name"},
		&cli.BoolFlag{Name: "clear-project", Usage: "remove the issue from its project"},
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
		if err := requireLinearAPIKey(); err != nil {
			return exitLinearError(stderr, err)
		}
		selector, err := linearTeamSelector(c)
		if err != nil {
			return exitLinearError(stderr, err)
		}
		client := newLinearClient()
		team, err := client.ResolveTeam(c.Context, selector)
		if err != nil {
			return exitLinearError(stderr, linearTeamResolutionError(err))
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

		input := linear.IssueCreateInput{Title: title, Description: description, TeamID: resolvedTeam.ID}
		if hasProject {
			input.ProjectID = resolvedProject.ID
		}
		if c.Bool("dry-run") {
			return writeLinearJSON(stdout, linearDryRunOutput{
				DryRun:    true,
				Operation: "create",
				Input:     input,
				Resolved: linearCreateResolvedOutput{
					Team:    resolvedTeam,
					Project: optionalLinearProject(resolvedProject, hasProject),
				},
			})
		}

		issue, err := client.CreateIssue(c.Context, input)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("failed to create issue", err))
		}
		output, err := newLinearIssueOutput(issue, resolvedTeam, optionalLinearProject(resolvedProject, hasProject))
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid created issue", err))
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
		team, err := client.ResolveTeam(c.Context, selector)
		if err != nil {
			return exitLinearError(stderr, linearTeamResolutionError(err))
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
		team, err := newLinearTeamOutput(issue.Team)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid issue response", err))
		}
		output, err := newLinearIssueOutput(issue, team, nil)
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
		if !hasLinearUpdate(input) && !projectChange.IsSet() {
			return exitLinearError(stderr, invalidLinearArguments(
				"at least --title, --description, --description-file, --clear-description, --project, or --clear-project required",
			))
		}
		if err := requireLinearAPIKey(); err != nil {
			return exitLinearError(stderr, err)
		}

		client := newLinearClient()
		input, resolvedProject, err := applyLinearProjectChange(
			c.Context,
			client,
			issueID,
			input,
			projectChange,
		)
		if err != nil {
			return exitLinearError(stderr, err)
		}

		if c.Bool("dry-run") {
			return writeLinearJSON(stdout, newLinearUpdateDryRunOutput(issueID, input, resolvedProject))
		}

		issue, err := client.UpdateIssue(c.Context, issueID, input)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("failed to update issue", err))
		}
		team, err := newLinearTeamOutput(issue.Team)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid issue response", err))
		}
		output, err := newLinearIssueOutput(issue, team, nil)
		if err != nil {
			return exitLinearError(stderr, linearAPIError("invalid issue response", err))
		}
		return writeLinearJSON(stdout, output)
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
	return input, nil
}

func hasLinearUpdate(input linear.UpdateIssueRequest) bool {
	return input.Title != nil || input.Description != nil || input.ClearDescription ||
		input.ProjectID != nil || input.ClearProject
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

func applyLinearProjectChange(
	ctx context.Context,
	client linear.Clienter,
	issueID string,
	input linear.UpdateIssueRequest,
	change linearProjectChange,
) (linear.UpdateIssueRequest, linearResolvedProjectChange, error) {
	if !change.IsSet() {
		return input, linearResolvedProjectChange{}, nil
	}
	if change.Clear {
		input.ClearProject = true
		return input, linearResolvedProjectChange{Changed: true}, nil
	}
	issue, err := client.GetIssue(ctx, issueID)
	if err != nil {
		return linear.UpdateIssueRequest{}, linearResolvedProjectChange{},
			linearAPIError("failed to get issue for project resolution", err)
	}
	team, err := newLinearTeamOutput(issue.Team)
	if err != nil {
		return linear.UpdateIssueRequest{}, linearResolvedProjectChange{}, linearAPIError("invalid issue response", err)
	}
	project, _, err := resolveLinearProject(ctx, client, team.ID, change.Selector, true)
	if err != nil {
		return linear.UpdateIssueRequest{}, linearResolvedProjectChange{}, err
	}
	input.ProjectID = &project.ID
	return input, linearResolvedProjectChange{Project: &project, Changed: true}, nil
}

func newLinearUpdateDryRunOutput(
	issueID string,
	input linear.UpdateIssueRequest,
	projectChange linearResolvedProjectChange,
) linearDryRunOutput {
	output := linearDryRunOutput{
		DryRun:    true,
		Operation: "update",
		Input:     linearUpdateDryRunInput{ID: issueID, Update: input},
	}
	if projectChange.Changed {
		output.Resolved = linearUpdateResolvedOutput{Project: projectChange.Project}
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
	project, err := client.ResolveProject(ctx, teamID, selector)
	if err != nil {
		return linearProjectOutput{}, false, linearProjectResolutionError(err)
	}
	output, err := newLinearProjectOutput(project)
	if err != nil {
		return linearProjectOutput{}, false, linearAPIError("invalid resolved project", err)
	}
	return output, true, nil
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
	var notFound *linear.TeamNotFoundError
	if errors.As(err, &notFound) {
		return &linearCommandError{Code: "team_not_found", Message: notFound.Error(), Err: err}
	}
	var ambiguous *linear.TeamAmbiguousError
	if errors.As(err, &ambiguous) {
		return &linearCommandError{
			Code:           "ambiguous_team",
			Message:        ambiguous.Error(),
			TeamCandidates: ambiguous.Candidates,
			Err:            err,
		}
	}
	return linearAPIError("failed to resolve team", err)
}

func linearProjectResolutionError(err error) error {
	var notFound *linear.ProjectNotFoundError
	if errors.As(err, &notFound) {
		return &linearCommandError{Code: "project_not_found", Message: notFound.Error(), Err: err}
	}
	var ambiguous *linear.ProjectAmbiguousError
	if errors.As(err, &ambiguous) {
		return &linearCommandError{
			Code:              "ambiguous_project",
			Message:           ambiguous.Error(),
			ProjectCandidates: ambiguous.Candidates,
			Err:               err,
		}
	}
	return linearAPIError("failed to resolve project", err)
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
	candidates := make([]any, 0, len(teamCandidates)+len(projectCandidates))
	for _, candidate := range teamCandidates {
		candidates = append(candidates, candidate)
	}
	for _, candidate := range projectCandidates {
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
) (linearIssueOutput, error) {
	id, ok := issue.ID.(string)
	if !ok || id == "" {
		return linearIssueOutput{}, fmt.Errorf("issue ID is not a string: %T", issue.ID)
	}
	project := fallbackProject
	if issue.Project.ID != nil {
		output, err := newLinearProjectOutput(issue.Project)
		if err != nil {
			return linearIssueOutput{}, err
		}
		project = &output
	}
	return linearIssueOutput{
		ID:          id,
		Identifier:  string(issue.Identifier),
		URL:         string(issue.URL),
		Title:       string(issue.Title),
		Description: string(issue.Description),
		Team:        team,
		Project:     project,
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

func newLinearTeamOutput(team linear.Team) (linearTeamOutput, error) {
	id, ok := team.ID.(string)
	if !ok || id == "" {
		return linearTeamOutput{}, fmt.Errorf("team ID is not a string: %T", team.ID)
	}
	return linearTeamOutput{ID: id, Key: string(team.Key), Name: string(team.Name)}, nil
}

func newLinearProjectOutput(project linear.Project) (linearProjectOutput, error) {
	id, ok := project.ID.(string)
	if !ok || id == "" {
		return linearProjectOutput{}, fmt.Errorf("project ID is not a string: %T", project.ID)
	}
	return linearProjectOutput{ID: id, Name: string(project.Name), SlugID: string(project.SlugID)}, nil
}

func optionalLinearProject(project linearProjectOutput, ok bool) *linearProjectOutput {
	if !ok {
		return nil
	}
	return &project
}
