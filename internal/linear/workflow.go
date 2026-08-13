package linear

import (
	"context"

	"github.com/shurcooL/graphql"
)

type WorkflowState struct {
	ID   graphql.ID     `graphql:"id"`
	Name graphql.String `graphql:"name"`
	Type graphql.String `graphql:"type"`
}

// RPC-style method to update issue state
func (c Client) UpdateIssueState(ctx context.Context, id string, stateID string) error {
	var m struct {
		IssueUpdate issuePayload `graphql:"issueUpdate(id: $id, input: $input)"`
	}
	variables := map[string]any{
		"id": graphql.String(id),
		"input": IssueUpdateInput{
			StateID: graphQLID(stateID),
		},
	}
	return c.client.Mutate(ctx, &m, variables)
}

// Query struct for getting workflow states
type workflowStatesQuery struct {
	WorkflowStates struct {
		Nodes []WorkflowState `graphql:"nodes"`
	} `graphql:"workflowStates(filter: {team: {id: {eq: $teamId}}})"`
}

// RPC-style method to get workflow states for a team
func (c Client) WorkflowStates(ctx context.Context, teamID string) ([]WorkflowState, error) {
	var q workflowStatesQuery
	variables := map[string]any{
		"teamId": graphQLID(teamID),
	}
	err := c.client.Query(ctx, &q, variables)
	if err != nil {
		return nil, err
	}
	return q.WorkflowStates.Nodes, nil
}
