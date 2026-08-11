package linear

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/shurcooL/graphql"
)

const apiURL = "https://api.linear.app/graphql"

type Clienter interface {
	CreateIssue(ctx context.Context, input IssueCreateInput) (Issue, error)
	GetIssue(ctx context.Context, id string) (Issue, error)
	ListLabels(ctx context.Context, teamID string, request LabelListRequest) (LabelPage, error)
	ListProjects(ctx context.Context, teamID string, request ProjectListRequest) (ProjectPage, error)
	ListTeams(ctx context.Context, request TeamListRequest) (TeamPage, error)
	ResolveLabel(ctx context.Context, teamID, selector string) (Label, error)
	ResolveProject(ctx context.Context, teamID string, selector string) (Project, error)
	ResolveTeam(ctx context.Context, selector string) (Team, error)
	UpdateIssue(ctx context.Context, id string, input UpdateIssueRequest) (Issue, error)
}

type APIError struct {
	Operation string
	Err       error
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Linear %s: %v", e.Operation, e.Err)
}

func (e *APIError) Unwrap() error {
	return e.Err
}

type Client struct {
	client *graphql.Client
}

func NewClient(token string) *Client {
	return newClient(token, apiURL)
}

func newClient(token, endpoint string) *Client {
	httpClient := &http.Client{
		Transport: &authTransport{token: token},
	}

	return &Client{
		client: graphql.NewClient(endpoint, httpClient),
	}
}

type authTransport struct {
	token string
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", t.token)
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultTransport.RoundTrip(req)
}

// GraphQL types for Linear API
type Issue struct {
	ID          graphql.ID     `graphql:"id"`
	Identifier  graphql.String `graphql:"identifier"`
	URL         graphql.String `graphql:"url"`
	Title       graphql.String `graphql:"title"`
	Description graphql.String `graphql:"description"`
	Assignee    User           `graphql:"assignee"`
	State       WorkflowState  `graphql:"state"`
	BranchName  graphql.String `graphql:"branchName"`
	Team        Team           `graphql:"team"`
	Project     Project        `graphql:"project"`
}

type User struct {
	ID    graphql.ID     `graphql:"id"`
	Name  graphql.String `graphql:"name"`
	Email graphql.String `graphql:"email"`
}

type Team struct {
	ID        graphql.ID     `graphql:"id"`
	Key       graphql.String `graphql:"key"`
	Name      graphql.String `graphql:"name"`
	RetiredAt graphql.String `graphql:"retiredAt"`
}

type Project struct {
	ID         graphql.ID     `graphql:"id"`
	Name       graphql.String `graphql:"name"`
	SlugID     graphql.String `graphql:"slugId"`
	ArchivedAt graphql.String `graphql:"archivedAt"`
}

type PageInfo struct {
	HasNextPage bool
	NextCursor  string
}

type TeamListRequest struct {
	Limit  int
	Cursor string
}

type TeamPage struct {
	Items    []Team
	PageInfo PageInfo
}

type ProjectListRequest struct {
	Limit  int
	Cursor string
}

type ProjectPage struct {
	Items    []Project
	PageInfo PageInfo
}

type Label struct {
	ID      graphql.ID      `graphql:"id"`
	Name    graphql.String  `graphql:"name"`
	IsGroup graphql.Boolean `graphql:"isGroup"`
	Team    Team            `graphql:"team"`
}

type LabelListRequest struct {
	Limit  int
	Cursor string
}

type LabelPage struct {
	Items    []Label
	PageInfo PageInfo
}

type TeamNotFoundError struct {
	Selector string
}

func (e *TeamNotFoundError) Error() string {
	return fmt.Sprintf("no active team matches %q", e.Selector)
}

type TeamAmbiguousError struct {
	Selector   string
	Candidates []Team
}

type LabelNotFoundError struct {
	Selector string
}

func (e *LabelNotFoundError) Error() string {
	return fmt.Sprintf("no applicable label matches %q", e.Selector)
}

type LabelAmbiguousError struct {
	Selector   string
	Candidates []Label
}

func (e *LabelAmbiguousError) Error() string {
	return fmt.Sprintf("multiple applicable labels match %q", e.Selector)
}

type LabelGroupError struct {
	Selector string
	Label    Label
}

func (e *LabelGroupError) Error() string {
	return fmt.Sprintf("label group %q cannot be applied to an issue", e.Selector)
}

func (e *TeamAmbiguousError) Error() string {
	return fmt.Sprintf("multiple active teams match %q", e.Selector)
}

type ProjectNotFoundError struct {
	TeamID   string
	Selector string
}

func (e *ProjectNotFoundError) Error() string {
	return fmt.Sprintf("no active project in team %q matches %q", e.TeamID, e.Selector)
}

type ProjectAmbiguousError struct {
	TeamID     string
	Selector   string
	Candidates []Project
}

func (e *ProjectAmbiguousError) Error() string {
	return fmt.Sprintf("multiple active projects in team %q match %q", e.TeamID, e.Selector)
}

type StringComparator struct {
	EqIgnoreCase *graphql.String `json:"eqIgnoreCase,omitempty"`
}

type TeamFilter struct {
	Key  *StringComparator `json:"key,omitempty"`
	Name *StringComparator `json:"name,omitempty"`
}

type IDComparator struct {
	Eq *graphql.ID `json:"eq,omitempty"`
}

type IssueLabelFilter struct {
	ID   *IDComparator     `json:"id,omitempty"`
	Name *StringComparator `json:"name,omitempty"`
}

type WorkflowState struct {
	ID   graphql.ID     `graphql:"id"`
	Name graphql.String `graphql:"name"`
	Type graphql.String `graphql:"type"`
}

// Query struct for getting an issue
type GetIssueQuery struct {
	Issue Issue `graphql:"issue(id: $id)"`
}

type teamConnection struct {
	Nodes    []Team          `graphql:"nodes"`
	PageInfo graphqlPageInfo `graphql:"pageInfo"`
}

type projectConnection struct {
	Nodes    []Project       `graphql:"nodes"`
	PageInfo graphqlPageInfo `graphql:"pageInfo"`
}

type graphqlPageInfo struct {
	HasNextPage graphql.Boolean `graphql:"hasNextPage"`
	EndCursor   graphql.String  `graphql:"endCursor"`
}

type teamsQuery struct {
	Teams teamConnection `graphql:"teams(first: $first, after: $after)"`
}

type filteredTeamsQuery struct {
	Teams teamConnection `graphql:"teams(first: $first, after: $after, filter: $filter)"`
}

type labelConnection struct {
	Nodes    []Label         `graphql:"nodes"`
	PageInfo graphqlPageInfo `graphql:"pageInfo"`
}

type labelsQuery struct {
	IssueLabels labelConnection `graphql:"issueLabels(first: $first, after: $after)"`
}

type filteredLabelsQuery struct {
	IssueLabels labelConnection `graphql:"issueLabels(first: $first, after: $after, filter: $filter)"`
}

type teamByIDQuery struct {
	Team Team `graphql:"team(id: $id)"`
}

type teamProjectsQuery struct {
	Team struct {
		Projects projectConnection `graphql:"projects(first: $first, after: $after)"`
	} `graphql:"team(id: $teamID)"`
}

// RPC-style method to get issue information
func (c *Client) GetIssue(ctx context.Context, id string) (Issue, error) {
	var q GetIssueQuery
	variables := map[string]any{
		"id": graphql.String(id),
	}
	err := c.client.Query(ctx, &q, variables)
	if err != nil {
		return Issue{}, &APIError{Operation: "get issue", Err: err}
	}
	if q.Issue.ID == nil {
		return Issue{}, &APIError{Operation: "get issue", Err: errors.New("issue not found")}
	}
	return q.Issue, nil
}

func (c *Client) ListTeams(ctx context.Context, request TeamListRequest) (TeamPage, error) {
	if request.Limit < 1 {
		return TeamPage{}, errors.New("team list limit must be greater than zero")
	}

	items := []Team{}
	cursor := request.Cursor
	pageInfo := PageInfo{}
	for len(items) < request.Limit {
		connection, err := c.queryTeams(ctx, request.Limit-len(items), cursor)
		if err != nil {
			return TeamPage{}, &APIError{Operation: "list teams", Err: err}
		}
		for _, team := range connection.Nodes {
			if len(items) < request.Limit && isActiveTeam(team) {
				items = append(items, team)
			}
		}
		pageInfo = newPageInfo(connection.PageInfo)
		if !pageInfo.HasNextPage {
			break
		}
		cursor = pageInfo.NextCursor
	}
	return TeamPage{Items: items, PageInfo: pageInfo}, nil
}

func (c *Client) ListProjects(ctx context.Context, teamID string, request ProjectListRequest) (ProjectPage, error) {
	if request.Limit < 1 {
		return ProjectPage{}, errors.New("project list limit must be greater than zero")
	}

	items := []Project{}
	cursor := request.Cursor
	pageInfo := PageInfo{}
	for len(items) < request.Limit {
		connection, err := c.queryTeamProjects(ctx, teamID, request.Limit-len(items), cursor)
		if err != nil {
			return ProjectPage{}, &APIError{Operation: "list projects", Err: err}
		}
		for _, project := range connection.Nodes {
			if len(items) < request.Limit && isActiveProject(project) {
				items = append(items, project)
			}
		}
		pageInfo = newPageInfo(connection.PageInfo)
		if !pageInfo.HasNextPage {
			break
		}
		cursor = pageInfo.NextCursor
	}
	return ProjectPage{Items: items, PageInfo: pageInfo}, nil
}

func (c *Client) ListLabels(ctx context.Context, teamID string, request LabelListRequest) (LabelPage, error) {
	if teamID == "" {
		return LabelPage{}, errors.New("label list team ID required")
	}
	if request.Limit < 1 {
		return LabelPage{}, errors.New("label list limit must be greater than zero")
	}

	items := []Label{}
	cursor := request.Cursor
	pageInfo := PageInfo{}
	for len(items) < request.Limit {
		connection, err := c.queryLabels(ctx, request.Limit-len(items), cursor)
		if err != nil {
			return LabelPage{}, &APIError{Operation: "list labels", Err: err}
		}
		for _, label := range connection.Nodes {
			if len(items) < request.Limit && isApplicableLabel(label, teamID) {
				items = append(items, label)
			}
		}
		pageInfo = newPageInfo(connection.PageInfo)
		if !pageInfo.HasNextPage {
			break
		}
		cursor = pageInfo.NextCursor
	}
	return LabelPage{Items: items, PageInfo: pageInfo}, nil
}

func (c *Client) ResolveTeam(ctx context.Context, selector string) (Team, error) {
	if selector == "" {
		return Team{}, &TeamNotFoundError{Selector: selector}
	}
	if isUUID(selector) {
		team, err := c.queryTeamByID(ctx, selector)
		if err != nil {
			return Team{}, &APIError{Operation: "resolve team", Err: err}
		}
		if isActiveTeam(team) {
			return team, nil
		}
	}

	keyMatches, err := c.queryTeamsByFilter(ctx, TeamFilter{Key: exactComparator(selector)})
	if err != nil {
		return Team{}, &APIError{Operation: "resolve team", Err: err}
	}
	if len(keyMatches) > 0 {
		return exactlyOneTeam(selector, keyMatches)
	}

	nameMatches, err := c.queryTeamsByFilter(ctx, TeamFilter{Name: exactComparator(selector)})
	if err != nil {
		return Team{}, &APIError{Operation: "resolve team", Err: err}
	}
	return exactlyOneTeam(selector, nameMatches)
}

func (c *Client) ResolveProject(ctx context.Context, teamID string, selector string) (Project, error) {
	if selector == "" {
		return Project{}, &ProjectNotFoundError{TeamID: teamID, Selector: selector}
	}

	candidates := []Project{}
	cursor := ""
	for {
		connection, err := c.queryTeamProjects(ctx, teamID, 50, cursor)
		if err != nil {
			return Project{}, &APIError{Operation: "resolve project", Err: err}
		}
		for _, project := range connection.Nodes {
			if isActiveProject(project) && projectMatchesSelector(project, selector) {
				candidates = append(candidates, project)
			}
		}
		pageInfo := newPageInfo(connection.PageInfo)
		if !pageInfo.HasNextPage {
			break
		}
		cursor = pageInfo.NextCursor
	}
	return exactlyOneProject(teamID, selector, candidates)
}

func (c *Client) ResolveLabel(ctx context.Context, teamID, selector string) (Label, error) {
	if teamID == "" || selector == "" {
		return Label{}, &LabelNotFoundError{Selector: selector}
	}
	if isUUID(selector) {
		idMatches, err := c.queryLabelsByFilter(ctx, teamID, IssueLabelFilter{ID: exactIDComparator(selector)})
		if err != nil {
			return Label{}, &APIError{Operation: "resolve label", Err: err}
		}
		if len(idMatches) > 0 {
			return exactlyOneLabel(selector, idMatches)
		}
	}
	nameMatches, err := c.queryLabelsByFilter(ctx, teamID, IssueLabelFilter{Name: exactComparator(selector)})
	if err != nil {
		return Label{}, &APIError{Operation: "resolve label", Err: err}
	}
	return exactlyOneLabel(selector, nameMatches)
}

func (c *Client) queryTeams(ctx context.Context, limit int, cursor string) (teamConnection, error) {
	var query teamsQuery
	variables := map[string]any{
		"first": graphql.Int(limit),
		"after": newCursor(cursor),
	}
	if err := c.client.Query(ctx, &query, variables); err != nil {
		return teamConnection{}, err
	}
	return query.Teams, nil
}

func (c *Client) queryTeamProjects(
	ctx context.Context,
	teamID string,
	limit int,
	cursor string,
) (projectConnection, error) {
	var query teamProjectsQuery
	variables := map[string]any{
		"teamID": graphql.ID(teamID),
		"first":  graphql.Int(limit),
		"after":  newCursor(cursor),
	}
	if err := c.client.Query(ctx, &query, variables); err != nil {
		return projectConnection{}, err
	}
	return query.Team.Projects, nil
}

func (c *Client) queryLabels(ctx context.Context, limit int, cursor string) (labelConnection, error) {
	var query labelsQuery
	variables := map[string]any{
		"first": graphql.Int(limit),
		"after": newCursor(cursor),
	}
	if err := c.client.Query(ctx, &query, variables); err != nil {
		return labelConnection{}, err
	}
	return query.IssueLabels, nil
}

func (c *Client) queryTeamsByFilter(ctx context.Context, filter TeamFilter) ([]Team, error) {
	items := []Team{}
	cursor := ""
	for {
		var query filteredTeamsQuery
		variables := map[string]any{
			"first":  graphql.Int(50),
			"after":  newCursor(cursor),
			"filter": filter,
		}
		if err := c.client.Query(ctx, &query, variables); err != nil {
			return nil, err
		}
		for _, team := range query.Teams.Nodes {
			if isActiveTeam(team) {
				items = append(items, team)
			}
		}
		pageInfo := newPageInfo(query.Teams.PageInfo)
		if !pageInfo.HasNextPage {
			return items, nil
		}
		cursor = pageInfo.NextCursor
	}
}

func (c *Client) queryLabelsByFilter(
	ctx context.Context,
	teamID string,
	filter IssueLabelFilter,
) ([]Label, error) {
	items := []Label{}
	cursor := ""
	for {
		var query filteredLabelsQuery
		variables := map[string]any{
			"first":  graphql.Int(50),
			"after":  newCursor(cursor),
			"filter": filter,
		}
		if err := c.client.Query(ctx, &query, variables); err != nil {
			return nil, err
		}
		for _, label := range query.IssueLabels.Nodes {
			if isApplicableLabel(label, teamID) {
				items = append(items, label)
			}
		}
		pageInfo := newPageInfo(query.IssueLabels.PageInfo)
		if !pageInfo.HasNextPage {
			return items, nil
		}
		cursor = pageInfo.NextCursor
	}
}

func (c *Client) queryTeamByID(ctx context.Context, id string) (Team, error) {
	var query teamByIDQuery
	variables := map[string]any{"id": graphql.ID(id)}
	if err := c.client.Query(ctx, &query, variables); err != nil {
		return Team{}, err
	}
	return query.Team, nil
}

func exactlyOneTeam(selector string, candidates []Team) (Team, error) {
	switch len(candidates) {
	case 0:
		return Team{}, &TeamNotFoundError{Selector: selector}
	case 1:
		return candidates[0], nil
	default:
		return Team{}, &TeamAmbiguousError{Selector: selector, Candidates: candidates}
	}
}

func exactlyOneProject(teamID string, selector string, candidates []Project) (Project, error) {
	switch len(candidates) {
	case 0:
		return Project{}, &ProjectNotFoundError{TeamID: teamID, Selector: selector}
	case 1:
		return candidates[0], nil
	default:
		return Project{}, &ProjectAmbiguousError{TeamID: teamID, Selector: selector, Candidates: candidates}
	}
}

func exactlyOneLabel(selector string, candidates []Label) (Label, error) {
	switch len(candidates) {
	case 0:
		return Label{}, &LabelNotFoundError{Selector: selector}
	case 1:
		if candidates[0].IsGroup {
			return Label{}, &LabelGroupError{Selector: selector, Label: candidates[0]}
		}
		return candidates[0], nil
	default:
		return Label{}, &LabelAmbiguousError{Selector: selector, Candidates: candidates}
	}
}

func exactComparator(value string) *StringComparator {
	selector := graphql.String(value)
	return &StringComparator{EqIgnoreCase: &selector}
}

func exactIDComparator(value string) *IDComparator {
	selector := graphql.ID(value)
	return &IDComparator{Eq: &selector}
}

func newCursor(cursor string) *graphql.String {
	if cursor == "" {
		return nil
	}
	value := graphql.String(cursor)
	return &value
}

func newPageInfo(pageInfo graphqlPageInfo) PageInfo {
	return PageInfo{HasNextPage: bool(pageInfo.HasNextPage), NextCursor: string(pageInfo.EndCursor)}
}

func isActiveTeam(team Team) bool {
	return team.ID != nil && team.RetiredAt == ""
}

func isActiveProject(project Project) bool {
	return project.ID != nil && project.ArchivedAt == ""
}

func projectMatchesSelector(project Project, selector string) bool {
	if isUUID(selector) {
		id, ok := project.ID.(string)
		return ok && id == selector
	}
	return string(project.SlugID) == selector || strings.EqualFold(string(project.Name), selector)
}

func isApplicableLabel(label Label, teamID string) bool {
	if label.ID == nil {
		return false
	}
	if label.Team.ID == nil {
		return true
	}
	labelTeamID, ok := label.Team.ID.(string)
	return ok && labelTeamID == teamID
}

func isUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		switch {
		case index == 8 || index == 13 || index == 18 || index == 23:
			if character != '-' {
				return false
			}
		case (character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') ||
			(character >= 'A' && character <= 'F'):
		default:
			return false
		}
	}
	return true
}

// Query struct for getting current user
type ViewerQuery struct {
	Viewer User `graphql:"viewer"`
}

// IssuePayload is the return type for issueUpdate mutation
type IssuePayload struct {
	Issue   Issue           `graphql:"issue"`
	Success graphql.Boolean `graphql:"success"`
}

type IssueCreateInput struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	TeamID      string   `json:"teamId"`
	ProjectID   string   `json:"projectId,omitempty"`
	LabelIDs    []string `json:"labelIds,omitempty"`
}

type UpdateIssueRequest struct {
	Title            *string
	Description      *string
	ClearDescription bool
	ProjectID        *string
	ClearProject     bool
	AddedLabelIDs    []string
	RemovedLabelIDs  []string
	ClearLabels      bool
}

func (i UpdateIssueRequest) MarshalJSON() ([]byte, error) {
	input := map[string]any{}
	if i.Title != nil {
		input["title"] = *i.Title
	}
	if i.Description != nil {
		input["description"] = *i.Description
	}
	if i.ClearDescription {
		input["description"] = nil
	}
	if i.ProjectID != nil {
		input["projectId"] = *i.ProjectID
	}
	if i.ClearProject {
		input["projectId"] = nil
	}
	if len(i.AddedLabelIDs) > 0 {
		input["addedLabelIds"] = i.AddedLabelIDs
	}
	if len(i.RemovedLabelIDs) > 0 {
		input["removedLabelIds"] = i.RemovedLabelIDs
	}
	if i.ClearLabels {
		input["labelIds"] = []string{}
	}
	return json.Marshal(input)
}

type nullableString struct {
	Value *graphql.String
}

type nullableID struct {
	Value *graphql.ID
}

func (s nullableID) MarshalJSON() ([]byte, error) {
	if s.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*s.Value)
}

func (s nullableString) MarshalJSON() ([]byte, error) {
	if s.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*s.Value)
}

type IssueUpdateInput struct {
	Title           *graphql.String `json:"title,omitempty"`
	Description     *nullableString `json:"description,omitempty"`
	ProjectID       *nullableID     `json:"projectId,omitempty"`
	AssigneeID      graphql.ID      `json:"assigneeId,omitempty"`
	StateID         graphql.ID      `json:"stateId,omitempty"`
	AddedLabelIDs   []graphql.ID    `json:"addedLabelIds,omitempty"`
	RemovedLabelIDs []graphql.ID    `json:"removedLabelIds,omitempty"`
	LabelIDs        *[]graphql.ID   `json:"labelIds,omitempty"`
}

type IssueCreateMutation struct {
	Issue   Issue           `graphql:"issue"`
	Success graphql.Boolean `graphql:"success"`
}

// RPC-style method to get current user
func (c *Client) GetViewer(ctx context.Context) (User, error) {
	var q ViewerQuery
	err := c.client.Query(ctx, &q, nil)
	if err != nil {
		return User{}, err
	}
	return q.Viewer, nil
}

// RPC-style method to assign issue to user
func (c *Client) AssignIssue(ctx context.Context, id string, assigneeID string) error {
	// First get the issue to get team ID
	issue, err := c.GetIssue(ctx, id)
	if err != nil {
		return err
	}
	teamID := issue.Team.ID.(string)

	// Get workflow states for the team
	states, err := c.GetWorkflowStates(ctx, teamID)
	if err != nil {
		return err
	}

	// Find "In Progress" state
	var inProgressID graphql.ID
	for _, state := range states {
		if string(state.Name) == "In Progress" {
			inProgressID = state.ID
			break
		}
	}
	if inProgressID == "" {
		return errors.New("in progress state not found")
	}

	var m struct {
		IssueUpdate IssuePayload `graphql:"issueUpdate(id: $id, input: $input)"`
	}
	variables := map[string]any{
		"id": graphql.String(id),
		"input": IssueUpdateInput{
			AssigneeID: graphql.ID(assigneeID),
			StateID:    inProgressID,
		},
	}
	return c.client.Mutate(ctx, &m, variables)
}

// RPC-style method to update issue state
func (c *Client) UpdateIssueState(ctx context.Context, id string, stateID string) error {
	var m struct {
		IssueUpdate IssuePayload `graphql:"issueUpdate(id: $id, input: $input)"`
	}
	variables := map[string]any{
		"id": graphql.String(id),
		"input": IssueUpdateInput{
			StateID: graphql.ID(stateID),
		},
	}
	return c.client.Mutate(ctx, &m, variables)
}

// Query struct for getting issues assigned to current user
type AssignedIssuesQuery struct {
	Viewer struct {
		AssignedIssues struct {
			Nodes []Issue `graphql:"nodes"`
		} `graphql:"assignedIssues"`
	} `graphql:"viewer"`
}

// RPC-style method to get issues assigned to current user
func (c *Client) GetAssignedIssues(ctx context.Context) ([]Issue, error) {
	var q AssignedIssuesQuery
	err := c.client.Query(ctx, &q, nil)
	if err != nil {
		return nil, err
	}
	return q.Viewer.AssignedIssues.Nodes, nil
}

// Query struct for getting workflow states
type WorkflowStatesQuery struct {
	WorkflowStates struct {
		Nodes []WorkflowState `graphql:"nodes"`
	} `graphql:"workflowStates(filter: {team: {id: {eq: $teamId}}})"`
}

// RPC-style method to get workflow states for a team
func (c *Client) GetWorkflowStates(ctx context.Context, teamID string) ([]WorkflowState, error) {
	var q WorkflowStatesQuery
	variables := map[string]any{
		"teamId": graphql.ID(teamID),
	}
	err := c.client.Query(ctx, &q, variables)
	if err != nil {
		return nil, err
	}
	return q.WorkflowStates.Nodes, nil
}

func (c *Client) CreateIssue(ctx context.Context, input IssueCreateInput) (Issue, error) {
	var m struct {
		IssueCreate IssueCreateMutation `graphql:"issueCreate(input: $input)"`
	}
	variables := map[string]any{
		"input": input,
	}
	if err := c.client.Mutate(ctx, &m, variables); err != nil {
		return Issue{}, &APIError{Operation: "create issue", Err: err}
	}
	if !m.IssueCreate.Success {
		return Issue{}, &APIError{Operation: "create issue", Err: errors.New("mutation was unsuccessful")}
	}
	return m.IssueCreate.Issue, nil
}

func (c *Client) UpdateIssue(ctx context.Context, id string, input UpdateIssueRequest) (Issue, error) {
	var m struct {
		IssueUpdate IssuePayload `graphql:"issueUpdate(id: $id, input: $input)"`
	}
	variables := map[string]any{
		"id":    graphql.String(id),
		"input": newIssueUpdateInput(input),
	}
	if err := c.client.Mutate(ctx, &m, variables); err != nil {
		return Issue{}, &APIError{Operation: "update issue", Err: err}
	}
	if !m.IssueUpdate.Success {
		return Issue{}, &APIError{Operation: "update issue", Err: errors.New("mutation was unsuccessful")}
	}
	return m.IssueUpdate.Issue, nil
}

func newIssueUpdateInput(input UpdateIssueRequest) IssueUpdateInput {
	output := IssueUpdateInput{}
	if input.Title != nil {
		title := graphql.String(*input.Title)
		output.Title = &title
	}
	if input.Description != nil {
		description := graphql.String(*input.Description)
		output.Description = &nullableString{Value: &description}
	}
	if input.ClearDescription {
		output.Description = &nullableString{}
	}
	if input.ProjectID != nil {
		projectID := graphql.ID(*input.ProjectID)
		output.ProjectID = &nullableID{Value: &projectID}
	}
	if input.ClearProject {
		output.ProjectID = &nullableID{}
	}
	output.AddedLabelIDs = newGraphQLIDs(input.AddedLabelIDs)
	output.RemovedLabelIDs = newGraphQLIDs(input.RemovedLabelIDs)
	if input.ClearLabels {
		labelIDs := []graphql.ID{}
		output.LabelIDs = &labelIDs
	}
	return output
}

func newGraphQLIDs(values []string) []graphql.ID {
	if len(values) == 0 {
		return nil
	}
	ids := make([]graphql.ID, 0, len(values))
	for _, value := range values {
		ids = append(ids, graphql.ID(value))
	}
	return ids
}
