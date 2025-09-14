package linear

import (
	"context"
	"net/http"

	"github.com/shurcooL/graphql"
)

type Client struct {
	client *graphql.Client
}

func NewClient(token string) Client {
	httpClient := &http.Client{
		Transport: &authTransport{token: token},
	}

	return Client{
		client: graphql.NewClient("https://api.linear.app/graphql", httpClient),
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
	Identifier  graphql.ID     `graphql:"identifier"`
	Title       graphql.String `graphql:"title"`
	Description graphql.String `graphql:"description"`
	Assignee    User           `graphql:"assignee"`
	State       WorkflowState  `graphql:"state"`
	BranchName  graphql.String `graphql:"branchName"`
}

type User struct {
	ID    graphql.ID     `graphql:"id"`
	Name  graphql.String `graphql:"name"`
	Email graphql.String `graphql:"email"`
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

// RPC-style method to get issue information
func (c *Client) GetIssue(ctx context.Context, id string) (Issue, error) {
	var q GetIssueQuery
	variables := map[string]any{
		"id": graphql.String(id),
	}
	err := c.client.Query(ctx, &q, variables)
	if err != nil {
		return Issue{}, err
	}
	return q.Issue, nil
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

// Mutation struct for assigning issue
type AssignIssueMutation struct {
	IssueUpdate IssuePayload `graphql:"issueUpdate(id: $id, input: $input)"`
}

type IssueUpdateInput struct {
	AssigneeID graphql.ID `json:"assigneeId,omitempty"`
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
	var m AssignIssueMutation
	variables := map[string]any{
		"id": graphql.String(id),
		"input": IssueUpdateInput{
			AssigneeID: graphql.ID(assigneeID),
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
