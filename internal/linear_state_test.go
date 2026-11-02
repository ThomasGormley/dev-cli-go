package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	linear "github.com/thomasgormley/dev-cli-go/internal/linear"
)

func TestHandleLinearGetReportsState(t *testing.T) {
	done := newLinearIssue()
	done.State = newDoneState()
	tests := []struct {
		name      string
		issue     linear.Issue
		wantState string
	}{
		{name: "with state", issue: done, wantState: `{"id":"state-done","name":"Done","type":"completed"}`},
		{name: "without state", issue: newLinearIssue(), wantState: "null"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := runLinearGet(t, &fakeLinearClient{issue: tt.issue})
			if err != nil {
				t.Fatalf("get returned error: %v (stderr %s)", err, stderr)
			}
			var output map[string]json.RawMessage
			if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
				t.Fatalf("decode issue output: %v", err)
			}
			if string(output["state"]) != tt.wantState {
				t.Errorf("expected state %s, got %s", tt.wantState, output["state"])
			}
		})
	}
}

func TestHandleLinearUpdateSetsStateWithinIssueTeam(t *testing.T) {
	issue := newLinearIssue()
	issue.State = newDoneState()
	client := &fakeLinearClient{issue: issue, resolveState: newDoneState()}

	stdout, stderr, err := runLinearUpdate(t, client, "--state", "done", "DEV-123")
	if err != nil {
		t.Fatalf("update returned error: %v (stderr %s)", err, stderr)
	}
	if client.resolveStateTeam != "team-uuid" || client.resolveStateTerm != "done" {
		t.Errorf("expected state resolved in the issue team, got team=%q selector=%q",
			client.resolveStateTeam, client.resolveStateTerm)
	}
	if want := (linear.UpdateIssueRequest{StateID: "state-done", SetState: true}); !reflect.DeepEqual(
		client.updateInput, want,
	) {
		t.Errorf("expected update input %+v, got %+v", want, client.updateInput)
	}
	var output linearIssueOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode issue output: %v", err)
	}
	if output.State == nil || *output.State != newDoneStateOutput() {
		t.Errorf("expected Done state in output, got %+v", output.State)
	}
}

func TestHandleLinearUpdateRejectsUnresolvedStateWithoutWriting(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode string
	}{
		{name: "no match", wantCode: "state_not_found"},
		{name: "ambiguous", err: errors.New(`multiple workflow states match "Done"`), wantCode: "ambiguous_state"},
		{name: "api failure", err: &linear.APIError{Operation: "resolve workflow state", Err: errors.New("boom")},
			wantCode: "api_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeLinearClient{issue: newLinearIssue(), resolveStateErr: tt.err}

			stdout, stderr, err := runLinearUpdate(t, client, "--title", "Renamed", "--state", "Done", "DEV-123")
			if err == nil {
				t.Fatal("expected a non-zero exit error")
			}
			assertLinearErrorCode(t, stderr.Bytes(), tt.wantCode)
			if client.updateCalls != 0 || stdout.Len() != 0 {
				t.Errorf("expected no writes and no stdout, got updates=%d stdout=%q", client.updateCalls, stdout)
			}
		})
	}
}

func TestHandleLinearUpdateRejectsEmptyStateBeforeCallingLinear(t *testing.T) {
	client := &fakeLinearClient{issue: newLinearIssue()}

	_, stderr, err := runLinearUpdate(t, client, "--state", "", "DEV-123")
	if err == nil {
		t.Fatal("expected a non-zero exit error")
	}
	assertLinearErrorCode(t, stderr.Bytes(), "invalid_arguments")
	if client.getID != "" || client.resolveStateTerm != "" || client.updateCalls != 0 {
		t.Errorf("expected no client calls, got get=%q state=%q updates=%d",
			client.getID, client.resolveStateTerm, client.updateCalls)
	}
}

func TestHandleLinearUpdateDryRunReportsResolvedState(t *testing.T) {
	client := &fakeLinearClient{issue: newLinearIssue(), resolveState: newDoneState()}

	stdout, stderr, err := runLinearUpdate(t, client, "--state", "Done", "--dry-run", "DEV-123")
	if err != nil {
		t.Fatalf("dry run returned error: %v (stderr %s)", err, stderr)
	}
	if client.updateCalls != 0 {
		t.Errorf("dry run must not update the issue, got %d update calls", client.updateCalls)
	}
	var output struct {
		Input    map[string]any             `json:"input"`
		Resolved linearUpdateResolvedOutput `json:"resolved"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode dry-run output: %v", err)
	}
	if want := map[string]any{"id": "DEV-123", "stateId": "state-done"}; !reflect.DeepEqual(output.Input, want) {
		t.Errorf("expected dry-run input %v, got %v", want, output.Input)
	}
	if output.Resolved.State == nil || *output.Resolved.State != newDoneStateOutput() {
		t.Errorf("expected resolved Done state, got %+v", output.Resolved.State)
	}
}

func TestHandleLinearStateList(t *testing.T) {
	client := &fakeLinearClient{
		resolveTeam: linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"},
		statePage: linear.WorkflowStatePage{
			Items:    []linear.WorkflowState{newDoneState()},
			PageInfo: linear.PageInfo{HasNextPage: true, NextCursor: "next-cursor"},
		},
	}

	stdout, stderr, err := runLinearStateList(t, client, "--team", "DEV", "--limit", "25", "--cursor", "previous-cursor")
	if err != nil {
		t.Fatalf("list returned error: %v (stderr %s)", err, stderr)
	}
	if client.resolveTerm != "DEV" || client.stateListTeam != "team-uuid" ||
		client.stateRequest != (linear.WorkflowStateListRequest{Limit: 25, Cursor: "previous-cursor"}) {
		t.Errorf("unexpected state list calls: team=%q teamID=%q request=%+v",
			client.resolveTerm, client.stateListTeam, client.stateRequest)
	}
	var output linearWorkflowStateListOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode state list output: %v", err)
	}
	want := linearWorkflowStateListOutput{
		Items:    []linearWorkflowStateOutput{newDoneStateOutput()},
		PageInfo: linearPageInfoOutput{HasNextPage: true, NextCursor: "next-cursor"},
	}
	if !reflect.DeepEqual(output, want) {
		t.Errorf("expected state list %+v, got %+v", want, output)
	}
}

func TestHandleLinearStateListRejectsNonPositiveLimit(t *testing.T) {
	client := &fakeLinearClient{resolveTeam: linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}}

	_, stderr, err := runLinearStateList(t, client, "--team", "DEV", "--limit", "0")
	if err == nil {
		t.Fatal("expected a non-zero exit error")
	}
	assertLinearErrorCode(t, stderr.Bytes(), "invalid_arguments")
	if client.resolveTerm != "" || client.stateListTeam != "" {
		t.Errorf("expected no client calls, got team=%q list=%q", client.resolveTerm, client.stateListTeam)
	}
}

func newDoneState() linear.WorkflowState {
	return linear.WorkflowState{ID: "state-done", Name: "Done", Type: "completed"}
}

func newDoneStateOutput() linearWorkflowStateOutput {
	return linearWorkflowStateOutput{ID: "state-done", Name: "Done", Type: "completed"}
}

func runLinearStateList(t *testing.T, client *fakeLinearClient, args ...string) (*bytes.Buffer, *bytes.Buffer, error) {
	t.Helper()
	t.Setenv("LINEAR_API_KEY", "token")
	withLinearClient(t, client)
	var stdout, stderr bytes.Buffer
	err := runLinearCommand(t, handleLinearStateList(&stdout, &stderr), linearStateListFlags(), args)
	return &stdout, &stderr, err
}
