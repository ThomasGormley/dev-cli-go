package linear

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/shurcooL/graphql"
)

// GraphQL types for Linear API
type Issue struct {
	ID               graphql.ID       `graphql:"id"`
	Identifier       graphql.String   `graphql:"identifier"`
	URL              graphql.String   `graphql:"url"`
	Title            graphql.String   `graphql:"title"`
	Description      graphql.String   `graphql:"description"`
	Assignee         User             `graphql:"assignee"`
	Priority         graphql.Int      `graphql:"priority"`
	State            WorkflowState    `graphql:"state"`
	BranchName       graphql.String   `graphql:"branchName"`
	Team             Team             `graphql:"team"`
	Project          Project          `graphql:"project"`
	ProjectMilestone ProjectMilestone `graphql:"projectMilestone"`
	Labels           LabelConnection  `graphql:"labels(first: 50)"`
}

type findIssueQuery struct {
	Issues issueConnection `graphql:"issues(first: 1, filter: $filter)"`
}

type issueConnection struct {
	Nodes []Issue `graphql:"nodes"`
}

// IssueFilter must keep Linear's GraphQL input type name because shurcooL/graphql
// derives variable declarations from Go type names.
type IssueFilter struct {
	Team   *TeamFilter       `json:"team,omitempty"`
	Number *numberComparator `json:"number,omitempty"`
}

func (c Client) FindIssue(ctx context.Context, id string) (Issue, bool, error) {
	teamKey, number, ok := parseIssueIdentifier(id)
	if !ok {
		return Issue{}, false, nil
	}
	var q findIssueQuery
	variables := map[string]any{
		"filter": IssueFilter{
			Team:   &TeamFilter{Key: exactComparator(teamKey)},
			Number: exactNumberComparator(number),
		},
	}
	err := c.client.Query(ctx, &q, variables)
	if err != nil {
		return Issue{}, false, &APIError{Operation: "find issue", Err: err}
	}
	if len(q.Issues.Nodes) == 0 {
		return Issue{}, false, nil
	}
	return q.Issues.Nodes[0], true, nil
}

func parseIssueIdentifier(value string) (string, int, bool) {
	key, numberText, ok := strings.Cut(strings.TrimSpace(value), "-")
	if !ok || key == "" || numberText == "" {
		return "", 0, false
	}
	number, err := strconv.Atoi(numberText)
	if err != nil || number < 1 {
		return "", 0, false
	}
	return key, number, true
}

type issuePayload struct {
	Issue   Issue           `graphql:"issue"`
	Success graphql.Boolean `graphql:"success"`
}

type IssueCreateInput struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	TeamID             string   `json:"teamId"`
	ProjectID          string   `json:"projectId,omitempty"`
	ProjectMilestoneID string   `json:"projectMilestoneId,omitempty"`
	LabelIDs           []string `json:"labelIds,omitempty"`
	AssigneeID         string   `json:"assigneeId,omitempty"`
	Priority           int      `json:"priority,omitempty"`
}

type UpdateIssueRequest struct {
	Title                 string
	SetTitle              bool
	Description           string
	SetDescription        bool
	ClearDescription      bool
	ProjectID             string
	SetProject            bool
	ClearProject          bool
	ProjectMilestoneID    string
	SetProjectMilestone   bool
	ClearProjectMilestone bool
	AssigneeID            string
	SetAssignee           bool
	ClearAssignee         bool
	Priority              int
	SetPriority           bool
	AddedLabelIDs         []string
	RemovedLabelIDs       []string
	ClearLabels           bool
}

func (i UpdateIssueRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(newIssueUpdateInput(i))
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

type nullableID struct {
	Value *graphql.ID
}

func (id nullableID) MarshalJSON() ([]byte, error) {
	if id.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*id.Value)
}

// IssueUpdateInput must keep Linear's GraphQL input type name because shurcooL/graphql
// derives variable declarations from Go type names.
type IssueUpdateInput struct {
	Title              *graphql.String `json:"title,omitempty"`
	Description        *nullableString `json:"description,omitempty"`
	ProjectID          *nullableID     `json:"projectId,omitempty"`
	ProjectMilestoneID *nullableID     `json:"projectMilestoneId,omitempty"`
	AssigneeID         *nullableID     `json:"assigneeId,omitempty"`
	Priority           *graphql.Int    `json:"priority,omitempty"`
	StateID            graphql.ID      `json:"stateId,omitempty"`
	AddedLabelIDs      []graphql.ID    `json:"addedLabelIds,omitempty"`
	RemovedLabelIDs    []graphql.ID    `json:"removedLabelIds,omitempty"`
	LabelIDs           *[]graphql.ID   `json:"labelIds,omitempty"`
}

type issueCreateMutation struct {
	Issue   Issue           `graphql:"issue"`
	Success graphql.Boolean `graphql:"success"`
}

func (c Client) AssignIssue(ctx context.Context, id string, assigneeID string) error {
	issue, found, err := c.FindIssue(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("issue not found")
	}
	teamID, ok := graphQLIDString(issue.Team.ID)
	if !ok {
		return errors.New("issue team ID missing")
	}

	states, err := c.WorkflowStates(ctx, teamID)
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
		IssueUpdate issuePayload `graphql:"issueUpdate(id: $id, input: $input)"`
	}
	variables := map[string]any{
		"id": graphql.String(id),
		"input": IssueUpdateInput{
			AssigneeID: newNullableID(assigneeID),
			StateID:    inProgressID,
		},
	}
	return c.client.Mutate(ctx, &m, variables)
}

type assignedIssuesQuery struct {
	Viewer struct {
		AssignedIssues struct {
			Nodes []Issue `graphql:"nodes"`
		} `graphql:"assignedIssues"`
	} `graphql:"viewer"`
}

func (c Client) AssignedIssues(ctx context.Context) ([]Issue, error) {
	var q assignedIssuesQuery
	err := c.client.Query(ctx, &q, nil)
	if err != nil {
		return nil, err
	}
	return q.Viewer.AssignedIssues.Nodes, nil
}

func (c Client) CreateIssue(ctx context.Context, input IssueCreateInput) (Issue, error) {
	var m struct {
		IssueCreate issueCreateMutation `graphql:"issueCreate(input: $input)"`
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

func (c Client) UpdateIssue(ctx context.Context, id string, input UpdateIssueRequest) (Issue, error) {
	var m struct {
		IssueUpdate issuePayload `graphql:"issueUpdate(id: $id, input: $input)"`
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
	if input.SetTitle {
		title := graphQLString(input.Title)
		output.Title = &title
	}
	if input.SetDescription {
		output.Description = newNullableString(input.Description)
	}
	if input.ClearDescription {
		output.Description = &nullableString{}
	}
	if input.SetProject {
		output.ProjectID = newNullableID(input.ProjectID)
	}
	if input.ClearProject {
		output.ProjectID = &nullableID{}
	}
	if input.SetAssignee {
		output.AssigneeID = newNullableID(input.AssigneeID)
	}
	if input.ClearAssignee {
		output.AssigneeID = &nullableID{}
	}
	if input.SetPriority {
		priority := graphQLInt(input.Priority)
		output.Priority = &priority
	}
	output.AddedLabelIDs = newGraphQLIDs(input.AddedLabelIDs)
	output.RemovedLabelIDs = newGraphQLIDs(input.RemovedLabelIDs)
	if input.ClearLabels {
		labelIDs := []graphql.ID{}
		output.LabelIDs = &labelIDs
	}
	if input.SetProjectMilestone {
		output.ProjectMilestoneID = newNullableID(input.ProjectMilestoneID)
	}
	if input.ClearProjectMilestone {
		output.ProjectMilestoneID = &nullableID{}
	}
	return output
}

func newNullableString(value string) *nullableString {
	graphQLValue := graphQLString(value)
	return &nullableString{Value: &graphQLValue}
}

func newNullableID(value string) *nullableID {
	graphQLValue := graphQLID(value)
	return &nullableID{Value: &graphQLValue}
}

func newGraphQLIDs(values []string) []graphql.ID {
	if len(values) == 0 {
		return nil
	}
	ids := make([]graphql.ID, 0, len(values))
	for _, value := range values {
		ids = append(ids, graphQLID(value))
	}
	return ids
}
