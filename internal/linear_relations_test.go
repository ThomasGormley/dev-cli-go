package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	linear "github.com/thomasgormley/dev-cli-go/internal/linear"
)

func TestHandleLinearGetReportsParentAndBlockers(t *testing.T) {
	withLinks := newLinearIssue()
	withLinks.Parent = linear.IssueRef{ID: "parent-uuid", Identifier: "DEV-1"}
	withLinks.InverseRelations.Nodes = []linear.IssueRelation{
		{ID: "relation-blocks", Type: "blocks", Issue: linear.IssueRef{ID: "blocker-uuid", Identifier: "DEV-2"}},
		{ID: "relation-related", Type: "related", Issue: linear.IssueRef{ID: "related-uuid", Identifier: "DEV-3"}},
	}
	tests := []struct {
		name          string
		issue         linear.Issue
		wantParent    string
		wantBlockedBy string
	}{
		{
			name:          "parent and blocks links only",
			issue:         withLinks,
			wantParent:    `{"id":"parent-uuid","identifier":"DEV-1"}`,
			wantBlockedBy: `[{"id":"blocker-uuid","identifier":"DEV-2"}]`,
		},
		{name: "no links", issue: newLinearIssue(), wantParent: "null", wantBlockedBy: "[]"},
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
			if string(output["parent"]) != tt.wantParent || string(output["blockedBy"]) != tt.wantBlockedBy {
				t.Errorf("expected parent=%s blockedBy=%s, got parent=%s blockedBy=%s",
					tt.wantParent, tt.wantBlockedBy, output["parent"], output["blockedBy"])
			}
		})
	}
}

func TestHandleLinearGetRejectsTruncatedRelations(t *testing.T) {
	issue := newLinearIssue()
	issue.InverseRelations.PageInfo.HasNextPage = true

	stdout, stderr, err := runLinearGet(t, &fakeLinearClient{issue: issue})
	if err == nil {
		t.Fatal("expected a non-zero exit error")
	}
	assertLinearErrorCode(t, stderr.Bytes(), "api_error")
	if stdout.Len() != 0 {
		t.Errorf("expected no stdout, got %q", stdout)
	}
}

func TestHandleLinearCreateResolvesParent(t *testing.T) {
	client := newParentFixture()
	client.issue = newLinearIssue()

	_, stderr, err := runLinearCreate(t, client, "--title", "Example issue", "--team", "DEV", "--parent", "DEV-1")
	if err != nil {
		t.Fatalf("create returned error: %v (stderr %s)", err, stderr)
	}
	if client.createInput.ParentID != "parent-uuid" {
		t.Errorf("expected parent UUID in create input, got %+v", client.createInput)
	}
}

func TestHandleLinearCreateDryRunReportsResolvedParent(t *testing.T) {
	client := newParentFixture()

	stdout, stderr, err := runLinearCreate(t, client,
		"--title", "Example issue", "--team", "DEV", "--parent", "DEV-1", "--dry-run")
	if err != nil {
		t.Fatalf("dry run returned error: %v (stderr %s)", err, stderr)
	}
	if client.createCalls != 0 {
		t.Errorf("expected no create calls, got %d", client.createCalls)
	}
	var output struct {
		Input struct {
			ParentID string `json:"parentId"`
		} `json:"input"`
		Resolved struct {
			Parent *linearIssueRefOutput `json:"parent"`
		} `json:"resolved"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode dry-run output: %v", err)
	}
	if output.Input.ParentID != "parent-uuid" || output.Resolved.Parent == nil ||
		*output.Resolved.Parent != (linearIssueRefOutput{ID: "parent-uuid", Identifier: "DEV-1"}) {
		t.Errorf("unexpected dry-run parent: %+v", output)
	}
}

func TestHandleLinearCreateRejectsUnknownParent(t *testing.T) {
	client := newParentFixture()

	stdout, stderr, err := runLinearCreate(t, client,
		"--title", "Example issue", "--team", "DEV", "--parent", "DEV-404")
	if err == nil {
		t.Fatal("expected a non-zero exit error")
	}
	assertLinearErrorCode(t, stderr.Bytes(), "issue_not_found")
	if client.createCalls != 0 || stdout.Len() != 0 {
		t.Errorf("expected no create and no stdout, got calls=%d stdout=%q", client.createCalls, stdout)
	}
}

func TestHandleLinearUpdateSetsAndClearsParent(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want linear.UpdateIssueRequest
	}{
		{
			name: "sets parent",
			args: []string{"--parent", "DEV-1", "DEV-123"},
			want: linear.UpdateIssueRequest{ParentID: "parent-uuid", SetParent: true},
		},
		{
			name: "clears parent",
			args: []string{"--clear-parent", "DEV-123"},
			want: linear.UpdateIssueRequest{ClearParent: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newParentFixture()
			client.issue = newLinearIssue()

			_, stderr, err := runLinearUpdate(t, client, tt.args...)
			if err != nil {
				t.Fatalf("update returned error: %v (stderr %s)", err, stderr)
			}
			if !reflect.DeepEqual(client.updateInput, tt.want) {
				t.Errorf("expected update input %+v, got %+v", tt.want, client.updateInput)
			}
		})
	}
}

func TestHandleLinearUpdateRejectsInvalidLinkFlagsBeforeCallingLinear(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "set and clear parent", args: []string{"--parent", "DEV-1", "--clear-parent", "DEV-123"}},
		{name: "empty parent", args: []string{"--parent", "", "DEV-123"}},
		{name: "own parent", args: []string{"--parent", "dev-123", "DEV-123"}},
		{name: "empty blocker", args: []string{"--add-blocked-by", "", "DEV-123"}},
		{name: "blocked by itself", args: []string{"--add-blocked-by", "DEV-123", "DEV-123"}},
		{
			name: "same blocker added and removed",
			args: []string{"--add-blocked-by", "DEV-2", "--remove-blocked-by", "dev-2", "DEV-123"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newBlockedByFixture()

			_, stderr, err := runLinearUpdate(t, client, tt.args...)
			if err == nil {
				t.Fatal("expected an invalid arguments error")
			}
			assertLinearErrorCode(t, stderr.Bytes(), "invalid_arguments")
			if client.getID != "" || client.updateCalls != 0 ||
				len(client.relationCreates) != 0 || len(client.relationDeletes) != 0 {
				t.Errorf("expected no client calls, got get=%q updates=%d creates=%v deletes=%v",
					client.getID, client.updateCalls, client.relationCreates, client.relationDeletes)
			}
		})
	}
}

func TestHandleLinearUpdateAddsOnlyMissingBlockers(t *testing.T) {
	client := newBlockedByFixture()

	stdout, stderr, err := runLinearUpdate(t, client,
		"--add-blocked-by", "DEV-2", "--add-blocked-by", "DEV-4", "--add-blocked-by", "DEV-4", "DEV-123")
	if err != nil {
		t.Fatalf("update returned error: %v (stderr %s)", err, stderr)
	}
	want := []linear.IssueRelationCreateInput{{IssueID: "dev-4-uuid", RelatedIssueID: "issue-uuid", Type: "blocks"}}
	if !reflect.DeepEqual(client.relationCreates, want) {
		t.Errorf("expected relation creates %+v, got %+v", want, client.relationCreates)
	}
	if client.updateCalls != 0 || len(client.relationDeletes) != 0 {
		t.Errorf("expected only relation creates, got updates=%d deletes=%v",
			client.updateCalls, client.relationDeletes)
	}
	assertIssueIdentifier(t, stdout.Bytes(), "DEV-123")
}

func TestHandleLinearUpdateRemovesOnlyLinkedBlockers(t *testing.T) {
	client := newBlockedByFixture()

	stdout, stderr, err := runLinearUpdate(t, client,
		"--remove-blocked-by", "DEV-2", "--remove-blocked-by", "DEV-3", "--remove-blocked-by", "DEV-4", "DEV-123")
	if err != nil {
		t.Fatalf("update returned error: %v (stderr %s)", err, stderr)
	}
	if want := []string{"relation-2"}; !reflect.DeepEqual(client.relationDeletes, want) {
		t.Errorf("expected relation deletes %v, got %v", want, client.relationDeletes)
	}
	if client.updateCalls != 0 || len(client.relationCreates) != 0 {
		t.Errorf("expected only relation deletes, got updates=%d creates=%v",
			client.updateCalls, client.relationCreates)
	}
	assertIssueIdentifier(t, stdout.Bytes(), "DEV-123")
}

func TestHandleLinearUpdateRejectsUnknownBlockerWithoutWriting(t *testing.T) {
	client := newBlockedByFixture()

	stdout, stderr, err := runLinearUpdate(t, client,
		"--title", "Renamed", "--add-blocked-by", "DEV-4", "--add-blocked-by", "DEV-404", "DEV-123")
	if err == nil {
		t.Fatal("expected a non-zero exit error")
	}
	assertLinearErrorCode(t, stderr.Bytes(), "issue_not_found")
	if client.updateCalls != 0 || len(client.relationCreates) != 0 || stdout.Len() != 0 {
		t.Errorf("expected no writes and no stdout, got updates=%d creates=%v stdout=%q",
			client.updateCalls, client.relationCreates, stdout)
	}
}

func TestHandleLinearUpdateStopsAtFirstFailedLink(t *testing.T) {
	client := newBlockedByFixture()
	client.relationErr = errors.New("boom")

	stdout, stderr, err := runLinearUpdate(t, client,
		"--add-blocked-by", "DEV-4", "--add-blocked-by", "DEV-6", "DEV-123")
	if err == nil {
		t.Fatal("expected a non-zero exit error")
	}
	var output linearErrorOutput
	if err := json.Unmarshal(stderr.Bytes(), &output); err != nil {
		t.Fatalf("decode stderr JSON: %v", err)
	}
	if output.Error.Code != "api_error" || !strings.Contains(output.Error.Message, "DEV-4") {
		t.Errorf("expected api_error naming DEV-4, got %+v", output.Error)
	}
	if len(client.relationCreates) != 1 || stdout.Len() != 0 {
		t.Errorf("expected one attempted create and no stdout, got creates=%v stdout=%q",
			client.relationCreates, stdout)
	}
}

func TestHandleLinearUpdateDryRunPrintsPlannedRelations(t *testing.T) {
	client := newBlockedByFixture()

	stdout, stderr, err := runLinearUpdate(t, client,
		"--add-blocked-by", "DEV-2", "--add-blocked-by", "DEV-4", "--remove-blocked-by", "DEV-5",
		"--dry-run", "DEV-123")
	if err != nil {
		t.Fatalf("dry run returned error: %v (stderr %s)", err, stderr)
	}
	if client.updateCalls != 0 || len(client.relationCreates) != 0 || len(client.relationDeletes) != 0 {
		t.Errorf("expected no writes, got updates=%d creates=%v deletes=%v",
			client.updateCalls, client.relationCreates, client.relationDeletes)
	}
	var output struct {
		Relations json.RawMessage `json:"relations"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode dry-run output: %v", err)
	}
	want := `{"create":[{"issueId":"dev-4-uuid","relatedIssueId":"issue-uuid","type":"blocks"}],` +
		`"delete":[{"id":"relation-5"}]}`
	if string(output.Relations) != want {
		t.Errorf("expected planned relations %s, got %s", want, output.Relations)
	}
}

func newParentFixture() *fakeLinearClient {
	return &fakeLinearClient{
		resolveTeam: linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"},
		issues:      map[string]linear.Issue{"DEV-1": {ID: "parent-uuid", Identifier: "DEV-1"}},
	}
}

// newBlockedByFixture returns DEV-123 blocked by DEV-2 and DEV-5 and related to DEV-3. DEV-4 and DEV-6
// are not linked.
func newBlockedByFixture() *fakeLinearClient {
	issue := newLinearIssue()
	issue.InverseRelations.Nodes = []linear.IssueRelation{
		{ID: "relation-2", Type: "blocks", Issue: linear.IssueRef{ID: "dev-2-uuid", Identifier: "DEV-2"}},
		{ID: "relation-3", Type: "related", Issue: linear.IssueRef{ID: "dev-3-uuid", Identifier: "DEV-3"}},
		{ID: "relation-5", Type: "blocks", Issue: linear.IssueRef{ID: "dev-5-uuid", Identifier: "DEV-5"}},
	}
	return &fakeLinearClient{issues: map[string]linear.Issue{
		"DEV-123": issue,
		"DEV-2":   {ID: "dev-2-uuid", Identifier: "DEV-2"},
		"DEV-3":   {ID: "dev-3-uuid", Identifier: "DEV-3"},
		"DEV-4":   {ID: "dev-4-uuid", Identifier: "DEV-4"},
		"DEV-5":   {ID: "dev-5-uuid", Identifier: "DEV-5"},
		"DEV-6":   {ID: "dev-6-uuid", Identifier: "DEV-6"},
	}}
}

func runLinearGet(t *testing.T, client *fakeLinearClient) (*bytes.Buffer, *bytes.Buffer, error) {
	t.Helper()
	t.Setenv("LINEAR_API_KEY", "token")
	withLinearClient(t, client)
	var stdout, stderr bytes.Buffer
	err := runLinearCommand(t, handleLinearGet(&stdout, &stderr), nil, []string{"DEV-123"})
	return &stdout, &stderr, err
}

func runLinearCreate(t *testing.T, client *fakeLinearClient, args ...string) (*bytes.Buffer, *bytes.Buffer, error) {
	t.Helper()
	t.Setenv("LINEAR_API_KEY", "token")
	withLinearClient(t, client)
	var stdout, stderr bytes.Buffer
	err := runLinearCommand(t, handleLinearCreate(&stdout, &stderr, bytes.NewReader(nil)), linearCreateFlags(), args)
	return &stdout, &stderr, err
}

func runLinearUpdate(t *testing.T, client *fakeLinearClient, args ...string) (*bytes.Buffer, *bytes.Buffer, error) {
	t.Helper()
	t.Setenv("LINEAR_API_KEY", "token")
	withLinearClient(t, client)
	var stdout, stderr bytes.Buffer
	err := runLinearCommand(t, handleLinearUpdate(&stdout, &stderr, bytes.NewReader(nil)), linearUpdateFlags(), args)
	return &stdout, &stderr, err
}

func assertIssueIdentifier(t *testing.T, data []byte, want string) {
	t.Helper()
	var output linearIssueOutput
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("decode issue output: %v; stdout=%q", err, string(data))
	}
	if output.Identifier != want {
		t.Errorf("expected issue %s in output, got %q", want, output.Identifier)
	}
}
