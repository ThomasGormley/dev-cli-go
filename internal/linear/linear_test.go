package linear

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
}

func TestClientGetIssueReturnsTypedAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(`{"errors":[{"message":"not found"}]}`))
		if err != nil {
			t.Errorf("write GraphQL error response: %v", err)
		}
	}))
	defer server.Close()

	_, err := newClient("test-token", server.URL).GetIssue(context.Background(), "DEV-404")
	if err == nil {
		t.Fatal("expected API error")
	}
	if _, ok := err.(*APIError); !ok {
		t.Errorf("expected APIError, got %T", err)
	}
}

func TestClientRejectsMissingIssueAndUnsuccessfulMutations(t *testing.T) {
	tests := []struct {
		name     string
		response string
		action   func(*Client) error
	}{
		{
			name:     "missing issue",
			response: `{"data":{"issue":null}}`,
			action: func(client *Client) error {
				_, err := client.GetIssue(context.Background(), "DEV-404")
				return err
			},
		},
		{
			name:     "unsuccessful create",
			response: `{"data":{"issueCreate":{"success":false,"issue":null}}}`,
			action: func(client *Client) error {
				_, err := client.CreateIssue(context.Background(), IssueCreateInput{Title: "Example", TeamID: "team-uuid"})
				return err
			},
		},
		{
			name:     "unsuccessful update",
			response: `{"data":{"issueUpdate":{"success":false,"issue":null}}}`,
			action: func(client *Client) error {
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
