package linear

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/shurcooL/graphql"
)

const issueUpdateResponse = `{
  "data": {
    "issueUpdate": {
      "success": true,
      "issue": {
        "id": "issue-uuid",
        "identifier": "DEV-123",
        "url": "https://linear.app/issue/DEV-123",
        "title": "Example",
        "description": ""
      }
    }
  }
}`

const (
	labelPageLimit          = 2
	remainingLabelPageLimit = 1
	testPriorityNone        = 0
)

func TestClientFindIssueReturnsLabels(t *testing.T) {
	requestReceived := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode GraphQL request: %v", err)
			return
		}
		requestReceived <- request.Query
		_, err := w.Write([]byte(`{"data":{"issues":{"nodes":[{"id":"issue-uuid","labels":{"nodes":[{"id":"label-bug","name":"Bug","isGroup":false,"team":{"id":"team-uuid","key":"DEV","name":"Development"}}]}}]}}}`))
		if err != nil {
			t.Errorf("write GraphQL response: %v", err)
		}
	}))
	defer server.Close()

	issue, found, err := newClient("test-token", server.URL).FindIssue(context.Background(), "DEV-123")
	if err != nil {
		t.Fatalf("get issue: %v", err)
	}
	if !found {
		t.Fatal("expected issue to be found")
	}
	if len(issue.Labels.Nodes) != 1 || issue.Labels.Nodes[0].ID != "label-bug" ||
		issue.Labels.Nodes[0].Name != "Bug" {
		t.Errorf("unexpected issue labels: %+v", issue.Labels.Nodes)
	}
	if query := <-requestReceived; !strings.Contains(query, "labels(first:") {
		t.Errorf("expected issue query to request labels, got %s", query)
	}
}

func TestClientUpdateIssueClearsDescription(t *testing.T) {
	requestReceived := make(chan struct {
		Authorization string
		Query         string
		Input         map[string]any
	}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			Query     string `json:"query"`
			Variables struct {
				Input map[string]any `json:"input"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode GraphQL request: %v", err)
			return
		}
		requestReceived <- struct {
			Authorization string
			Query         string
			Input         map[string]any
		}{Authorization: r.Header.Get("Authorization"), Query: request.Query, Input: request.Variables.Input}

		_, err := w.Write([]byte(issueUpdateResponse))
		if err != nil {
			t.Errorf("write GraphQL response: %v", err)
		}
	}))
	defer server.Close()

	client := newClient("test-token", server.URL)
	issue, err := client.UpdateIssue(context.Background(), "DEV-123", UpdateIssueRequest{ClearDescription: true})
	if err != nil {
		t.Fatalf("update issue: %v", err)
	}
	if issue.ID.(string) != "issue-uuid" {
		t.Errorf("expected returned issue UUID, got %v", issue.ID)
	}

	request := <-requestReceived
	if request.Authorization != "test-token" {
		t.Errorf("expected Authorization header, got %q", request.Authorization)
	}
	if request.Query == "" {
		t.Error("expected a GraphQL mutation")
	}
	value, ok := request.Input["description"]
	if !ok || value != nil {
		t.Errorf("expected an explicit null description, got %#v", request.Input)
	}
	if _, ok := request.Input["title"]; ok {
		t.Errorf("expected title to be omitted, got %#v", request.Input)
	}
	if _, ok := request.Input["projectId"]; ok {
		t.Errorf("expected project ID to be unchanged, got %#v", request.Input)
	}
}

func TestClientUpdateIssueClearsProject(t *testing.T) {
	requestReceived := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			Variables struct {
				Input map[string]any `json:"input"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode GraphQL request: %v", err)
			return
		}
		requestReceived <- request.Variables.Input
		_, err := w.Write([]byte(issueUpdateResponse))
		if err != nil {
			t.Errorf("write GraphQL response: %v", err)
		}
	}))
	defer server.Close()

	_, err := newClient("test-token", server.URL).UpdateIssue(
		context.Background(),
		"DEV-123",
		UpdateIssueRequest{ClearProject: true},
	)
	if err != nil {
		t.Fatalf("update issue: %v", err)
	}

	input := <-requestReceived
	value, ok := input["projectId"]
	if !ok || value != nil {
		t.Errorf("expected an explicit null projectId, got %#v", input)
	}
}

func TestClientCreateIssueIncludesProject(t *testing.T) {
	requestReceived := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			Variables struct {
				Input map[string]any `json:"input"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode GraphQL request: %v", err)
			return
		}
		requestReceived <- request.Variables.Input
		_, err := w.Write([]byte(`{"data":{"issueCreate":{"success":true,"issue":{"id":"issue-uuid"}}}}`))
		if err != nil {
			t.Errorf("write GraphQL response: %v", err)
		}
	}))
	defer server.Close()

	_, err := newClient("test-token", server.URL).CreateIssue(context.Background(), IssueCreateInput{
		Title: "Example", TeamID: "team-uuid", ProjectID: "project-uuid",
	})
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}
	input := <-requestReceived
	if input["projectId"] != "project-uuid" {
		t.Errorf("expected project ID in mutation input, got %#v", input)
	}
}

func TestClientCreateIssueIncludesProjectMilestone(t *testing.T) {
	requestReceived := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			Variables struct {
				Input map[string]any `json:"input"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode GraphQL request: %v", err)
			return
		}
		requestReceived <- request.Variables.Input
		_, err := w.Write([]byte(`{"data":{"issueCreate":{"success":true,"issue":{"id":"issue-uuid"}}}}`))
		if err != nil {
			t.Errorf("write GraphQL response: %v", err)
		}
	}))
	defer server.Close()

	_, err := newClient("test-token", server.URL).CreateIssue(context.Background(), IssueCreateInput{
		Title: "Example", TeamID: "team-uuid", ProjectID: "project-uuid", ProjectMilestoneID: "milestone-uuid",
	})
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}
	input := <-requestReceived
	if input["projectMilestoneId"] != "milestone-uuid" {
		t.Errorf("expected project milestone ID in mutation input, got %#v", input)
	}
}

func TestClientUpdateIssueClearsProjectMilestone(t *testing.T) {
	requestReceived := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			Variables struct {
				Input map[string]any `json:"input"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode GraphQL request: %v", err)
			return
		}
		requestReceived <- request.Variables.Input
		_, err := w.Write([]byte(issueUpdateResponse))
		if err != nil {
			t.Errorf("write GraphQL response: %v", err)
		}
	}))
	defer server.Close()

	_, err := newClient("test-token", server.URL).UpdateIssue(
		context.Background(),
		"DEV-123",
		UpdateIssueRequest{ClearProjectMilestone: true},
	)
	if err != nil {
		t.Fatalf("update issue: %v", err)
	}

	input := <-requestReceived
	value, ok := input["projectMilestoneId"]
	if !ok || value != nil {
		t.Errorf("expected an explicit null projectMilestoneId, got %#v", input)
	}
}

func TestClientListAndFindProjectMilestones(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		requests++
		response := `{
  "data": {
    "project": {
      "projectMilestones": {
        "nodes": [
          {"id":"milestone-one","name":"Launch","targetDate":"2026-08-20","status":"planned","project":{"id":"project-uuid"}}
        ],
        "pageInfo": {"hasNextPage": false, "endCursor": ""}
      }
    }
  }
}`
		if requests == 1 {
			response = `{
  "data": {
    "project": {
      "projectMilestones": {
        "nodes": [
          {"id":"milestone-one","name":"Launch","targetDate":"2026-08-20","status":"planned","project":{"id":"project-uuid"}}
        ],
        "pageInfo": {"hasNextPage": true, "endCursor": "next-cursor"}
      }
    }
  }
}`
		}
		if requests == 2 {
			response = `{
  "data": {
    "project": {
      "projectMilestones": {
        "nodes": [
          {"id":"milestone-two","name":"Ship","targetDate":"2026-08-21","status":"planned","project":{"id":"project-uuid"}}
        ],
        "pageInfo": {"hasNextPage": false, "endCursor": ""}
      }
    }
  }
}`
		}
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := newClient("test-token", server.URL)
	page, err := client.ListProjectMilestones(context.Background(), "project-uuid", ProjectMilestoneListRequest{Limit: 10})
	if err != nil {
		t.Fatalf("list milestones: %v", err)
	}
	if len(page.Items) != 2 || page.Items[0].ID != "milestone-one" || page.Items[1].ID != "milestone-two" ||
		page.Items[0].Project.ID != "project-uuid" {
		t.Errorf("unexpected milestone page: %+v", page)
	}
	milestone, found, err := client.FindProjectMilestone(context.Background(), "project-uuid", "launch")
	if err != nil {
		t.Fatalf("resolve milestone: %v", err)
	}
	if !found || milestone.ID != "milestone-one" || milestone.Name != "Launch" || requests != 3 {
		t.Errorf("unexpected resolved milestone: %+v, requests=%d", milestone, requests)
	}
}

func TestClientListProjectsPaginatesAndExcludesArchivedProjects(t *testing.T) {
	requests := make([]struct {
		First  int    `json:"first"`
		After  string `json:"after"`
		TeamID string `json:"teamID"`
	}, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			Variables struct {
				First  int    `json:"first"`
				After  string `json:"after"`
				TeamID string `json:"teamID"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		requests = append(requests, request.Variables)
		response := `{
  "data": {
    "team": {
      "projects": {
        "nodes": [
          {"id":"project-one","name":"One","slugId":"one","archivedAt":null},
          {"id":"project-archived","name":"Old","slugId":"old","archivedAt":"2025-01-01"}
        ],
        "pageInfo": {"hasNextPage":true,"endCursor":"cursor-one"}
      }
    }
  }
}`
		if len(requests) == 2 {
			response = `{
  "data": {
    "team": {
      "projects": {
        "nodes": [{"id":"project-two","name":"Two","slugId":"two","archivedAt":null}],
        "pageInfo": {"hasNextPage":true,"endCursor":"cursor-two"}
      }
    }
  }
}`
		}
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	page, err := newClient("test-token", server.URL).ListProjects(
		context.Background(),
		"team-uuid",
		ProjectListRequest{Limit: 2},
	)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(page.Items) != 2 || page.Items[0].ID != "project-one" || page.Items[1].ID != "project-two" {
		t.Errorf("unexpected projects: %+v", page.Items)
	}
	if !page.PageInfo.HasNextPage || page.PageInfo.NextCursor != "cursor-two" {
		t.Errorf("unexpected page info: %+v", page.PageInfo)
	}
	if len(requests) != 2 || requests[0].First != 2 || requests[0].After != "" ||
		requests[1].First != 1 || requests[1].After != "cursor-one" {
		t.Errorf("unexpected pagination requests: %+v", requests)
	}
}

func TestClientFindProjectRequiresOneTeamScopedExactMatch(t *testing.T) {
	tests := []struct {
		name      string
		selector  string
		response  string
		wantFound bool
		wantErr   bool
		wantID    string
		wantName  string
		wantCalls int
	}{
		{
			name:     "resolves exact slug",
			selector: "agent-work",
			response: `{
  "data": {"team": {"projects": {
    "nodes": [{"id":"project-uuid","name":"Agent work","slugId":"agent-work","archivedAt":null}],
    "pageInfo": {"hasNextPage":false,"endCursor":""}
  }}}
			}`,
			wantFound: true,
			wantName:  "Agent work",
		},
		{
			name:     "resolves exact case insensitive name",
			selector: "AGENT WORK",
			response: `{
  "data": {"team": {"projects": {
    "nodes": [{"id":"project-uuid","name":"Agent work","slugId":"agent-work","archivedAt":null}],
    "pageInfo": {"hasNextPage":false,"endCursor":""}
  }}}
}`,
			wantFound: true,
			wantName:  "Agent work",
		},
		{
			name:     "rejects ambiguous exact names",
			selector: "Agent work",
			response: `{
  "data": {"team": {"projects": {
    "nodes": [
      {"id":"project-one","name":"Agent work","slugId":"agent-work-one","archivedAt":null},
      {"id":"project-two","name":"Agent work","slugId":"agent-work-two","archivedAt":null}
    ],
    "pageInfo": {"hasNextPage":false,"endCursor":""}
  }}}
}`,
			wantErr: true,
		},
		{
			name:     "reports missing project",
			selector: "missing",
			response: `{"data":{"team":{"projects":{"nodes":[],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if _, err := w.Write([]byte(tt.response)); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()

			project, found, err := newClient("test-token", server.URL).FindProject(
				context.Background(), "team-uuid", tt.selector,
			)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected resolution error")
				}
				return
			}
			if tt.wantFound {
				if err != nil {
					t.Fatalf("resolve project: %v", err)
				}
				if !found {
					t.Fatal("expected project")
				}
				wantID := tt.wantID
				if wantID == "" {
					wantID = "project-uuid"
				}
				if project.ID != wantID || project.Name != graphql.String(tt.wantName) {
					t.Errorf("unexpected project: %+v", project)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve project: %v", err)
			}
			if found {
				t.Fatalf("expected no project, got %+v", project)
			}
		})
	}
}

func TestClientFindIssueReturnsTypedAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(`{"errors":[{"message":"not found"}]}`))
		if err != nil {
			t.Errorf("write GraphQL error response: %v", err)
		}
	}))
	defer server.Close()

	_, _, err := newClient("test-token", server.URL).FindIssue(context.Background(), "DEV-404")
	if err == nil {
		t.Fatal("expected API error")
	}
	if _, ok := err.(*APIError); !ok {
		t.Errorf("expected APIError, got %T", err)
	}
}

func TestClientFindIssueReportsNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(`{"data":{"issues":{"nodes":[]}}}`))
		if err != nil {
			t.Errorf("write GraphQL response: %v", err)
		}
	}))
	defer server.Close()

	issue, found, err := newClient("test-token", server.URL).FindIssue(context.Background(), "DEV-404")
	if err != nil {
		t.Fatalf("find issue: %v", err)
	}
	if found {
		t.Fatalf("expected no issue, got %+v", issue)
	}
}

func TestClientRejectsUnsuccessfulMutations(t *testing.T) {
	tests := []struct {
		name     string
		response string
		action   func(Client) error
	}{
		{
			name:     "unsuccessful create",
			response: `{"data":{"issueCreate":{"success":false,"issue":null}}}`,
			action: func(client Client) error {
				_, err := client.CreateIssue(context.Background(), IssueCreateInput{Title: "Example", TeamID: "team-uuid"})
				return err
			},
		},
		{
			name:     "unsuccessful update",
			response: `{"data":{"issueUpdate":{"success":false,"issue":null}}}`,
			action: func(client Client) error {
				_, err := client.UpdateIssue(context.Background(), "DEV-123", UpdateIssueRequest{})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, err := w.Write([]byte(tt.response))
				if err != nil {
					t.Errorf("write GraphQL response: %v", err)
				}
			}))
			defer server.Close()

			err := tt.action(newClient("test-token", server.URL))
			if err == nil {
				t.Fatal("expected API error")
			}
			if _, ok := err.(*APIError); !ok {
				t.Errorf("expected APIError, got %T", err)
			}
		})
	}
}

func TestClientListTeamsPaginatesAndExcludesRetiredTeams(t *testing.T) {
	requests := make([]struct {
		First int    `json:"first"`
		After string `json:"after"`
	}, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			Variables struct {
				First int    `json:"first"`
				After string `json:"after"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		requests = append(requests, request.Variables)
		response := `{"data":{"teams":{"nodes":[{"id":"team-one","key":"ONE","name":"One","retiredAt":null},{"id":"team-retired","key":"OLD","name":"Old","retiredAt":"2025-01-01"}],"pageInfo":{"hasNextPage":true,"endCursor":"cursor-one"}}}}`
		if len(requests) == 2 {
			response = `{"data":{"teams":{"nodes":[{"id":"team-two","key":"TWO","name":"Two","retiredAt":null}],"pageInfo":{"hasNextPage":true,"endCursor":"cursor-two"}}}}`
		}
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	page, err := newClient("test-token", server.URL).ListTeams(context.Background(), TeamListRequest{Limit: 2})
	if err != nil {
		t.Fatalf("list teams: %v", err)
	}
	if len(page.Items) != 2 || string(page.Items[0].ID.(string)) != "team-one" ||
		string(page.Items[1].ID.(string)) != "team-two" {
		t.Errorf("unexpected teams: %+v", page.Items)
	}
	if !page.PageInfo.HasNextPage || page.PageInfo.NextCursor != "cursor-two" {
		t.Errorf("unexpected page info: %+v", page.PageInfo)
	}
	if len(requests) != 2 || requests[0].First != 2 || requests[0].After != "" ||
		requests[1].First != 1 || requests[1].After != "cursor-one" {
		t.Errorf("unexpected pagination requests: %+v", requests)
	}
}

func TestClientFindTeamUsesKeyBeforeExactName(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			Query     string `json:"query"`
			Variables struct {
				Filter struct {
					Key struct {
						EqIgnoreCase string `json:"eqIgnoreCase"`
					} `json:"key"`
				} `json:"filter"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		requests++
		if !strings.Contains(request.Query, "teams(first: $first, after: $after, filter: $filter)") {
			t.Errorf("unexpected query: %s", request.Query)
		}
		if request.Variables.Filter.Key.EqIgnoreCase != "platform" {
			t.Errorf("expected exact key selector, got %+v", request.Variables.Filter)
		}
		_, err := w.Write([]byte(`{"data":{"teams":{"nodes":[{"id":"team-uuid","key":"PLATFORM","name":"Platform","retiredAt":null}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`))
		if err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	team, found, err := newClient("test-token", server.URL).FindTeam(context.Background(), "platform")
	if err != nil {
		t.Fatalf("resolve team: %v", err)
	}
	if !found {
		t.Fatal("expected team")
	}
	if team.ID != "team-uuid" || team.Key != "PLATFORM" || team.Name != "Platform" {
		t.Errorf("unexpected team: %+v", team)
	}
	if requests != 1 {
		t.Errorf("expected one key query, got %d", requests)
	}
}

func TestClientFindTeamReportsNotFoundAndAmbiguity(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantErr  bool
	}{
		{
			name:     "not found",
			response: `{"data":{"teams":{"nodes":[],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`,
		},
		{
			name:     "ambiguous exact name",
			response: `{"data":{"teams":{"nodes":[{"id":"team-one","key":"ONE","name":"Platform","retiredAt":null},{"id":"team-two","key":"TWO","name":"Platform","retiredAt":null}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				response := tt.response
				if calls == 1 {
					response = `{"data":{"teams":{"nodes":[],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`
				}
				if _, err := w.Write([]byte(response)); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()

			team, found, err := newClient("test-token", server.URL).FindTeam(context.Background(), "Platform")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected resolution error")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve team: %v", err)
			}
			if found {
				t.Fatalf("expected no team, got %+v", team)
			}
		})
	}
}

func TestClientListLabelsPaginatesApplicableTeamAndWorkspaceLabels(t *testing.T) {
	requests := make([]struct {
		First int    `json:"first"`
		After string `json:"after"`
	}, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			Variables struct {
				First int    `json:"first"`
				After string `json:"after"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		requests = append(requests, request.Variables)
		response := `{
			"data": {"issueLabels": {"nodes": [
				{"id":"team-label","name":"Team","isGroup":false,"team":{"id":"team-one","key":"ONE","name":"One"}},
				{"id":"other-label","name":"Other","isGroup":false,"team":{"id":"team-two","key":"TWO","name":"Two"}}
			],"pageInfo":{"hasNextPage":true,"endCursor":"cursor-one"}}}
		}`
		if len(requests) == labelPageLimit {
			response = `{
				"data": {"issueLabels": {"nodes": [
					{"id":"workspace-label","name":"Workspace","isGroup":false,"team":null}
				],"pageInfo":{"hasNextPage":true,"endCursor":"cursor-two"}}}
			}`
		}
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	page, err := newClient("test-token", server.URL).ListLabels(
		context.Background(),
		"team-one",
		LabelListRequest{Limit: labelPageLimit},
	)
	if err != nil {
		t.Fatalf("list labels: %v", err)
	}
	if len(page.Items) != labelPageLimit || page.Items[0].ID != "team-label" ||
		page.Items[1].ID != "workspace-label" {
		t.Errorf("unexpected labels: %+v", page.Items)
	}
	if !page.PageInfo.HasNextPage || page.PageInfo.NextCursor != "cursor-two" {
		t.Errorf("unexpected page info: %+v", page.PageInfo)
	}
	if len(requests) != labelPageLimit || requests[0].First != labelPageLimit || requests[0].After != "" ||
		requests[1].First != remainingLabelPageLimit || requests[1].After != "cursor-one" {
		t.Errorf("unexpected pagination requests: %+v", requests)
	}
}

func TestClientFindLabelRejectsAmbiguityAndGroups(t *testing.T) {
	tests := []struct {
		name     string
		response string
	}{
		{
			name: "team and workspace name conflict",
			response: `{
				"data": {"issueLabels": {"nodes": [
					{"id":"team-label","name":"Bug","isGroup":false,"team":{"id":"team-one","key":"ONE","name":"One"}},
					{"id":"workspace-label","name":"Bug","isGroup":false,"team":null}
				],"pageInfo":{"hasNextPage":false,"endCursor":""}}}
			}`,
		},
		{
			name: "label group",
			response: `{
				"data": {"issueLabels": {"nodes": [
					{"id":"group-label","name":"Type","isGroup":true,"team":null}
				],"pageInfo":{"hasNextPage":false,"endCursor":""}}}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer r.Body.Close()
				var request struct {
					Query string `json:"query"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Errorf("decode request: %v", err)
					return
				}
				if !strings.Contains(request.Query, "issueLabels(first: $first, after: $after, filter: $filter)") {
					t.Errorf("unexpected label query: %s", request.Query)
				}
				if _, err := w.Write([]byte(tt.response)); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()

			_, _, err := newClient("test-token", server.URL).FindLabel(
				context.Background(),
				"team-one",
				"Bug",
			)
			if err == nil {
				t.Fatal("expected resolution error")
			}
		})
	}
}

func TestClientListUsersPaginatesAndExcludesInactiveUsers(t *testing.T) {
	requests := make([]struct {
		First  int    `json:"first"`
		After  string `json:"after"`
		TeamID string `json:"teamID"`
	}, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			Variables struct {
				First  int    `json:"first"`
				After  string `json:"after"`
				TeamID string `json:"teamID"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		requests = append(requests, request.Variables)
		response := `{"data":{"team":{"members":{"nodes":[{"id":"user-one","name":"One","displayName":"One","email":"one@example.com","active":true},{"id":"user-inactive","name":"Inactive","displayName":"Inactive","email":"inactive@example.com","active":false}],"pageInfo":{"hasNextPage":true,"endCursor":"cursor-one"}}}}}`
		if len(requests) == 2 {
			response = `{"data":{"team":{"members":{"nodes":[{"id":"user-two","name":"Two","displayName":"Two","email":"two@example.com","active":true}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`
		}
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	page, err := newClient("test-token", server.URL).ListUsers(context.Background(), "team-uuid", UserListRequest{Limit: 2})
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(page.Items) != 2 || page.Items[0].ID != "user-one" || page.Items[1].ID != "user-two" {
		t.Errorf("unexpected users: %+v", page.Items)
	}
	if page.PageInfo.HasNextPage || page.PageInfo.NextCursor != "" {
		t.Errorf("unexpected page info: %+v", page.PageInfo)
	}
	if len(requests) != 2 || requests[0].First != 2 || requests[0].After != "" ||
		requests[1].First != 1 || requests[1].After != "cursor-one" {
		t.Errorf("unexpected pagination requests: %+v", requests)
	}
}

func TestClientFindAssignee(t *testing.T) {
	tests := []struct {
		name     string
		selector string
		response string
		wantErr  bool
	}{
		{
			name:     "exact email",
			selector: "ada@example.com",
			response: `{"data":{"team":{"members":{"nodes":[{"id":"user-ada","name":"Ada Lovelace","displayName":"Ada","email":"ada@example.com","active":true}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`,
		},
		{
			name:     "ambiguous exact name",
			selector: "Alex",
			response: `{"data":{"team":{"members":{"nodes":[{"id":"user-one","name":"Alex","displayName":"Alex","email":"one@example.com","active":true},{"id":"user-two","name":"Alex","displayName":"Alex","email":"two@example.com","active":true}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if _, err := w.Write([]byte(tt.response)); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()

			user, found, err := newClient("test-token", server.URL).FindAssignee(
				context.Background(),
				"team-uuid",
				tt.selector,
			)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("resolve assignee: %v", err)
				}
				if !found {
					t.Fatal("expected assignee")
				}
				if user.ID != "user-ada" || user.Email != "ada@example.com" {
					t.Errorf("unexpected resolved user: %+v", user)
				}
				return
			}
			if err == nil {
				t.Fatal("expected resolution error")
			}
		})
	}
}

func TestClientFindAssigneeMeRequiresAnActiveTeamMember(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		response := `{"data":{"viewer":{"id":"viewer-uuid","name":"Viewer","displayName":"Viewer","email":"viewer@example.com","active":true}}}`
		if requests == 2 {
			response = `{"data":{"team":{"members":{"nodes":[{"id":"viewer-uuid","name":"Viewer","displayName":"Viewer","email":"viewer@example.com","active":true}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`
		}
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	user, found, err := newClient("test-token", server.URL).FindAssignee(context.Background(), "team-uuid", "me")
	if err != nil {
		t.Fatalf("resolve me: %v", err)
	}
	if !found {
		t.Fatal("expected viewer")
	}
	if user.ID != "viewer-uuid" || requests != 2 {
		t.Errorf("unexpected resolved viewer: %+v, calls=%d", user, requests)
	}
}

func TestIssueMutationInputsIncludeAssigneeAndPriority(t *testing.T) {
	requestReceived := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			Variables struct {
				Input map[string]any `json:"input"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		requestReceived <- request.Variables.Input
		if _, err := w.Write([]byte(issueUpdateResponse)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	_, err := newClient("test-token", server.URL).UpdateIssue(context.Background(), "DEV-123", UpdateIssueRequest{
		AssigneeID:  "user-uuid",
		SetAssignee: true,
		Priority:    testPriorityNone,
		SetPriority: true,
	})
	if err != nil {
		t.Fatalf("update issue: %v", err)
	}
	request := <-requestReceived
	if request["assigneeId"] != "user-uuid" || request["priority"] != float64(testPriorityNone) {
		t.Errorf("expected assignee and priority mutation input, got %#v", request)
	}
}

func TestIssueUpdateInputClearsAssignee(t *testing.T) {
	data, err := json.Marshal(newIssueUpdateInput(UpdateIssueRequest{ClearAssignee: true}))
	if err != nil {
		t.Fatalf("marshal update request: %v", err)
	}
	var output map[string]any
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("decode update request: %v", err)
	}
	if value, ok := output["assigneeId"]; !ok || value != nil {
		t.Errorf("expected explicit null assignee, got %#v", output)
	}
}

func TestClientUpdateIssuePatchesAndClearsLabels(t *testing.T) {
	tests := []struct {
		name  string
		input UpdateIssueRequest
		want  map[string]any
	}{
		{
			name: "patches labels",
			input: UpdateIssueRequest{
				AddedLabelIDs:   []string{"label-add"},
				RemovedLabelIDs: []string{"label-remove"},
			},
			want: map[string]any{
				"addedLabelIds":   []any{"label-add"},
				"removedLabelIds": []any{"label-remove"},
			},
		},
		{
			name:  "clears labels",
			input: UpdateIssueRequest{ClearLabels: true},
			want:  map[string]any{"labelIds": []any{}},
		},
	}

	requests := make([]map[string]any, 0, len(tests))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			Variables struct {
				Input map[string]any `json:"input"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		requests = append(requests, request.Variables.Input)
		if _, err := w.Write([]byte(issueUpdateResponse)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := newClient("test-token", server.URL)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.UpdateIssue(context.Background(), "DEV-123", tt.input)
			if err != nil {
				t.Fatalf("update issue: %v", err)
			}
			got := requests[len(requests)-1]
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expected mutation input %#v, got %#v", tt.want, got)
			}
		})
	}
}
