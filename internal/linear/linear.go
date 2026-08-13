package linear

import (
	"context"
	"fmt"
	"net/http"

	"github.com/shurcooL/graphql"
)

const apiURL = "https://api.linear.app/graphql"

type Clienter interface {
	CreateIssue(ctx context.Context, input IssueCreateInput) (Issue, error)
	FindIssue(ctx context.Context, id string) (Issue, bool, error)
	ListLabels(ctx context.Context, teamID string, request LabelListRequest) (LabelPage, error)
	ListProjects(ctx context.Context, teamID string, request ProjectListRequest) (ProjectPage, error)
	ListProjectMilestones(
		ctx context.Context,
		projectID string,
		request ProjectMilestoneListRequest,
	) (ProjectMilestonePage, error)
	ListTeams(ctx context.Context, request TeamListRequest) (TeamPage, error)
	ListUsers(ctx context.Context, teamID string, request UserListRequest) (UserPage, error)
	FindLabel(ctx context.Context, teamID, selector string) (Label, bool, error)
	FindAssignee(ctx context.Context, teamID, selector string) (User, bool, error)
	FindProject(ctx context.Context, teamID string, selector string) (Project, bool, error)
	FindProjectMilestone(ctx context.Context, projectID string, selector string) (ProjectMilestone, bool, error)
	FindTeam(ctx context.Context, selector string) (Team, bool, error)
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

func NewClient(token string) Client {
	return newClient(token, apiURL)
}

func newClient(token, endpoint string) Client {
	httpClient := &http.Client{
		Transport: authTransport{token: token},
	}

	return Client{
		client: graphql.NewClient(endpoint, httpClient),
	}
}

type authTransport struct {
	token string
}

func (t authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", t.token)
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultTransport.RoundTrip(req)
}

func graphQLString(value string) graphql.String {
	return graphql.String(value)
}

func graphQLID(value string) graphql.ID {
	return graphql.ID(value)
}

func graphQLInt(value int) graphql.Int {
	return graphql.Int(value)
}

func graphQLIDString(value graphql.ID) (string, bool) {
	id, ok := value.(string)
	return id, ok && id != ""
}
