package linear

import (
	"context"
	"errors"
	"fmt"

	"github.com/shurcooL/graphql"
)

type WorkflowState struct {
	ID   graphql.ID     `graphql:"id"`
	Name graphql.String `graphql:"name"`
	Type graphql.String `graphql:"type"`
}

type WorkflowStateListRequest = Pagination

type WorkflowStatePage struct {
	Items    []WorkflowState
	PageInfo PageInfo
}

// WorkflowStateFilter must keep Linear's GraphQL input type name because shurcooL/graphql
// derives variable declarations from Go type names.
type WorkflowStateFilter struct {
	Name *stringComparator `json:"name,omitempty"`
}

type workflowStateConnection struct {
	Nodes    []WorkflowState `graphql:"nodes"`
	PageInfo graphqlPageInfo `graphql:"pageInfo"`
}

type teamWorkflowStatesQuery struct {
	Team struct {
		States workflowStateConnection `graphql:"states(first: $first, after: $after)"`
	} `graphql:"team(id: $teamID)"`
}

type filteredTeamWorkflowStatesQuery struct {
	Team struct {
		States workflowStateConnection `graphql:"states(first: $first, after: $after, filter: $filter)"`
	} `graphql:"team(id: $teamID)"`
}

func (c Client) ListWorkflowStates(
	ctx context.Context,
	teamID string,
	request WorkflowStateListRequest,
) (WorkflowStatePage, error) {
	if request.Limit < 1 {
		return WorkflowStatePage{}, errors.New("workflow state list limit must be greater than zero")
	}

	items := []WorkflowState{}
	cursor := request.Cursor
	pageInfo := PageInfo{}
	for len(items) < request.Limit {
		connection, err := c.queryTeamWorkflowStates(
			ctx,
			teamID,
			Pagination{Limit: request.Limit - len(items), Cursor: cursor},
		)
		if err != nil {
			return WorkflowStatePage{}, &APIError{Operation: "list workflow states", Err: err}
		}
		for _, state := range connection.Nodes {
			if len(items) < request.Limit {
				items = append(items, state)
			}
		}
		pageInfo = newPageInfo(connection.PageInfo)
		if !pageInfo.HasNextPage {
			break
		}
		cursor = pageInfo.NextCursor
	}
	return WorkflowStatePage{Items: items, PageInfo: pageInfo}, nil
}

func (c Client) FindWorkflowState(ctx context.Context, teamID, selector string) (WorkflowState, bool, error) {
	if selector == "" {
		return WorkflowState{}, false, nil
	}
	matches, err := c.queryWorkflowStatesByFilter(ctx, teamID, WorkflowStateFilter{Name: exactComparator(selector)})
	if err != nil {
		return WorkflowState{}, false, &APIError{Operation: "resolve workflow state", Err: err}
	}
	return selectWorkflowState(selector, matches)
}

func (c Client) queryTeamWorkflowStates(
	ctx context.Context,
	teamID string,
	pagination Pagination,
) (workflowStateConnection, error) {
	var query teamWorkflowStatesQuery
	variables := map[string]any{
		"teamID": graphQLString(teamID),
		"first":  graphql.Int(pagination.Limit),
		"after":  newCursor(pagination.Cursor),
	}
	if err := c.client.Query(ctx, &query, variables); err != nil {
		return workflowStateConnection{}, err
	}
	return query.Team.States, nil
}

func (c Client) queryWorkflowStatesByFilter(
	ctx context.Context,
	teamID string,
	filter WorkflowStateFilter,
) ([]WorkflowState, error) {
	items := []WorkflowState{}
	cursor := ""
	for {
		var query filteredTeamWorkflowStatesQuery
		variables := map[string]any{
			"teamID": graphQLString(teamID),
			"first":  graphql.Int(50),
			"after":  newCursor(cursor),
			"filter": filter,
		}
		if err := c.client.Query(ctx, &query, variables); err != nil {
			return nil, err
		}
		items = append(items, query.Team.States.Nodes...)
		pageInfo := newPageInfo(query.Team.States.PageInfo)
		if !pageInfo.HasNextPage {
			return items, nil
		}
		cursor = pageInfo.NextCursor
	}
}

func selectWorkflowState(selector string, candidates []WorkflowState) (WorkflowState, bool, error) {
	switch len(candidates) {
	case 0:
		return WorkflowState{}, false, nil
	case 1:
		return candidates[0], true, nil
	default:
		return WorkflowState{}, false, fmt.Errorf("multiple workflow states match %q", selector)
	}
}
