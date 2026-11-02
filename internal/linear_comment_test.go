package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	linear "github.com/thomasgormley/dev-cli-go/internal/linear"
)

func TestHandleLinearCommentPostsBodyToResolvedIssue(t *testing.T) {
	const body = "Closing: shipped in #42.\n"
	client := newCommentFixture()

	stdout, stderr, err := runLinearComment(t, client, strings.NewReader(body), "--body-file", "-", "DEV-123")
	if err != nil {
		t.Fatalf("comment returned error: %v (stderr %s)", err, stderr)
	}
	want := []linear.CommentCreateInput{{IssueID: "issue-uuid", Body: body}}
	if !reflect.DeepEqual(client.commentInputs, want) {
		t.Errorf("expected comment inputs %+v, got %+v", want, client.commentInputs)
	}
	var output linearCommentOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode comment output: %v", err)
	}
	wantOutput := linearCommentOutput{
		ID:    "comment-uuid",
		URL:   "https://linear.app/example/issue/DEV-123#comment-1",
		Body:  body,
		Issue: linearIssueRefOutput{ID: "issue-uuid", Identifier: "DEV-123"},
	}
	if output != wantOutput {
		t.Errorf("expected comment output %+v, got %+v", wantOutput, output)
	}
}

func TestHandleLinearCommentRejectsInvalidArgumentsBeforeCallingLinear(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "no body", args: []string{"DEV-123"}},
		{name: "body and body file", args: []string{"--body", "one", "--body-file", "-", "DEV-123"}},
		{name: "empty body", args: []string{"--body", "", "DEV-123"}},
		{name: "whitespace body", args: []string{"--body", " \n\t", "DEV-123"}},
		{name: "whitespace stdin", args: []string{"--body-file", "-", "DEV-123"}},
		{name: "no issue", args: []string{"--body", "Done"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newCommentFixture()

			stdout, stderr, err := runLinearComment(t, client, strings.NewReader("  \n"), tt.args...)
			if err == nil {
				t.Fatal("expected a non-zero exit error")
			}
			assertLinearErrorCode(t, stderr.Bytes(), "invalid_arguments")
			if client.getID != "" || len(client.commentInputs) != 0 || stdout.Len() != 0 {
				t.Errorf("expected no client calls and no stdout, got get=%q comments=%v stdout=%q",
					client.getID, client.commentInputs, stdout)
			}
		})
	}
}

func TestHandleLinearCommentRejectsUnknownIssueWithoutPosting(t *testing.T) {
	client := newCommentFixture()

	stdout, stderr, err := runLinearComment(t, client, nil, "--body", "Done", "DEV-404")
	if err == nil {
		t.Fatal("expected a non-zero exit error")
	}
	assertLinearErrorCode(t, stderr.Bytes(), "issue_not_found")
	if len(client.commentInputs) != 0 || stdout.Len() != 0 {
		t.Errorf("expected no comment and no stdout, got comments=%v stdout=%q", client.commentInputs, stdout)
	}
}

func TestHandleLinearCommentReportsAPIFailure(t *testing.T) {
	client := newCommentFixture()
	client.commentErr = &linear.APIError{Operation: "create comment", Err: errors.New("mutation was unsuccessful")}

	stdout, stderr, err := runLinearComment(t, client, nil, "--body", "Done", "DEV-123")
	if err == nil {
		t.Fatal("expected a non-zero exit error")
	}
	assertLinearErrorCode(t, stderr.Bytes(), "api_error")
	if stdout.Len() != 0 {
		t.Errorf("expected no stdout, got %q", stdout)
	}
}

func TestHandleLinearCommentDryRunDoesNotPost(t *testing.T) {
	client := newCommentFixture()

	stdout, stderr, err := runLinearComment(t, client, nil, "--body", "Done", "--dry-run", "DEV-123")
	if err != nil {
		t.Fatalf("dry run returned error: %v (stderr %s)", err, stderr)
	}
	if len(client.commentInputs) != 0 {
		t.Errorf("dry run must not post a comment, got %v", client.commentInputs)
	}
	var output struct {
		DryRun    bool                        `json:"dryRun"`
		Operation string                      `json:"operation"`
		Input     linear.CommentCreateInput   `json:"input"`
		Resolved  linearCommentResolvedOutput `json:"resolved"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode dry-run output: %v", err)
	}
	if !output.DryRun || output.Operation != "comment" ||
		output.Input != (linear.CommentCreateInput{IssueID: "issue-uuid", Body: "Done"}) ||
		output.Resolved.Issue != (linearIssueRefOutput{ID: "issue-uuid", Identifier: "DEV-123"}) {
		t.Errorf("unexpected dry-run output: %+v", output)
	}
}

func newCommentFixture() *fakeLinearClient {
	return &fakeLinearClient{
		issues: map[string]linear.Issue{"DEV-123": newLinearIssue()},
		comment: linear.Comment{
			ID:    "comment-uuid",
			URL:   "https://linear.app/example/issue/DEV-123#comment-1",
			Body:  "Closing: shipped in #42.\n",
			Issue: linear.IssueRef{ID: "issue-uuid", Identifier: "DEV-123"},
		},
	}
}

func runLinearComment(
	t *testing.T,
	client *fakeLinearClient,
	stdin io.Reader,
	args ...string,
) (*bytes.Buffer, *bytes.Buffer, error) {
	t.Helper()
	t.Setenv("LINEAR_API_KEY", "token")
	withLinearClient(t, client)
	if stdin == nil {
		stdin = bytes.NewReader(nil)
	}
	var stdout, stderr bytes.Buffer
	err := runLinearCommand(t, handleLinearComment(&stdout, &stderr, stdin), linearCommentFlags(), args)
	return &stdout, &stderr, err
}
