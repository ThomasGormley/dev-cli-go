package linear

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/shurcooL/graphql"
)

const apiURL = "https://api.linear.app/graphql"

type Clienter interface {
	CreateIssue(ctx context.Context, input IssueCreateInput) (Issue, error)
	GetIssue(ctx context.Context, id string) (Issue, error)
	ListTeams(ctx context.Context, request TeamListRequest) (TeamPage, error)
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

func (e *TeamAmbiguousError) Error() string {
	return fmt.Sprintf("multiple active teams match %q", e.Selector)
}

type StringComparator struct {
	EqIgnoreCase *graphql.String `json:"eqIgnoreCase,omitempty"`
}

type TeamFilter struct {
	Key  *StringComparator `json:"key,omitempty"`
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

type teamByIDQuery struct {
	Team Team `graphql:"team(id: $id)"`
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

func exactComparator(value string) *StringComparator {
	selector := graphql.String(value)
	return &StringComparator{EqIgnoreCase: &selector}
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
	Title       string `json:"title"`
	Description string `json:"description"`
	TeamID      string `json:"teamId"`
}

type UpdateIssueRequest struct {
	Title            *string
	Description      *string
	ClearDescription bool
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
	return json.Marshal(input)
}

type nullableString struct {
	Value *graphql.String
}

func (s nullableString) MarshalJSON() ([]byte, error) {
	if s.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*s.Value)
}

type IssueUpdateInput struct {
	Title       *graphql.String `json:"title,omitempty"`
	Description *nullableString `json:"description,omitempty"`
	AssigneeID  graphql.ID      `json:"assigneeId,omitempty"`
	StateID     graphql.ID      `json:"stateId,omitempty"`
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
	return output
}
