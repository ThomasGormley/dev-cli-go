package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/urfave/cli/v2"

	linear "github.com/thomasgormley/dev-cli-go/internal/linear"
)

type fakeLinearClient struct {
	createInput linear.IssueCreateInput
	getID       string
	updateID    string
	updateInput linear.UpdateIssueRequest
	issue       linear.Issue
	err         error
	createCalls int
	updateCalls int
}

func (f *fakeLinearClient) CreateIssue(_ context.Context, input linear.IssueCreateInput) (linear.Issue, error) {
	f.createCalls++
	f.createInput = input
	return f.issue, f.err
}

func (f *fakeLinearClient) GetIssue(_ context.Context, id string) (linear.Issue, error) {
	f.getID = id
	return f.issue, f.err
}

func (f *fakeLinearClient) UpdateIssue(
	_ context.Context,
	id string,
	input linear.UpdateIssueRequest,
) (linear.Issue, error) {
	f.updateCalls++
	f.updateID = id
	f.updateInput = input
	return f.issue, f.err
}

func newLinearIssue() linear.Issue {
	return linear.Issue{
		ID:          "issue-uuid",
		Identifier:  "DEV-123",
		URL:         "https://linear.app/example/issue/DEV-123/example",
		Title:       "Example issue",
		Description: "Example description",
	}
}

func withLinearClient(t *testing.T, client linear.Clienter) {
	t.Helper()
	previous := newLinearClient
	newLinearClient = func() linear.Clienter { return client }
	t.Cleanup(func() { newLinearClient = previous })
}

func TestHandleLinearCreateSuccess(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	t.Setenv("LINEAR_TEAM_ID", "team-uuid")
	client := &fakeLinearClient{issue: newLinearIssue()}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearCreate(&stdout, &stderr, bytes.NewBufferString("unused")),
		linearCreateFlags(),
		[]string{"--title", "Example issue", "--description", "Example description"},
	)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}
	if client.createInput != (linear.IssueCreateInput{
		Title: "Example issue", Description: "Example description", TeamID: "team-uuid",
	}) {
		t.Errorf("unexpected create input: %+v", client.createInput)
	}

	assertIssueOutput(t, stdout.Bytes())
	if stderr.Len() != 0 {
		t.Errorf("expected no stderr, got %q", stderr.String())
	}
}

func TestHandleLinearGetSuccess(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	client := &fakeLinearClient{issue: newLinearIssue()}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(t, handleLinearGet(&stdout, &stderr), nil, []string{"DEV-123"})
	if err != nil {
		t.Fatalf("get returned error: %v", err)
	}
	if client.getID != "DEV-123" {
		t.Errorf("expected issue ID DEV-123, got %q", client.getID)
	}
	assertIssueOutput(t, stdout.Bytes())
}

func TestHandleLinearUpdateSupportsDescriptionFileAndClear(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	client := &fakeLinearClient{issue: newLinearIssue()}
	withLinearClient(t, client)

	file := filepath.Join(t.TempDir(), "description.md")
	if err := os.WriteFile(file, []byte("from a file"), 0o600); err != nil {
		t.Fatalf("write description file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearUpdate(&stdout, &stderr, bytes.NewBufferString("from stdin")),
		linearUpdateFlags(),
		[]string{"--description-file", file, "DEV-123"},
	)
	if err != nil {
		t.Fatalf("update returned error: %v", err)
	}
	if client.updateID != "DEV-123" ||
		client.updateInput.Description == nil ||
		*client.updateInput.Description != "from a file" {
		t.Errorf("unexpected file update: id=%q input=%+v", client.updateID, client.updateInput)
	}
	assertIssueOutput(t, stdout.Bytes())

	stdout.Reset()
	stderr.Reset()
	err = runLinearCommand(
		t,
		handleLinearUpdate(&stdout, &stderr, bytes.NewBufferString("unused")),
		linearUpdateFlags(),
		[]string{"--clear-description", "DEV-123"},
	)
	if err != nil {
		t.Fatalf("clear description returned error: %v", err)
	}
	if !client.updateInput.ClearDescription || client.updateInput.Description != nil {
		t.Errorf("unexpected clear-description input: %+v", client.updateInput)
	}
}

func TestHandleLinearCreateDryRunDoesNotCallClient(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	t.Setenv("LINEAR_TEAM_ID", "team-uuid")
	client := &fakeLinearClient{}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearCreate(&stdout, &stderr, bytes.NewBufferString("body from stdin")),
		linearCreateFlags(),
		[]string{"--title", "Example issue", "--description-file", "-", "--dry-run"},
	)
	if err != nil {
		t.Fatalf("dry run returned error: %v", err)
	}
	if client.createCalls != 0 {
		t.Errorf("expected no create calls, got %d", client.createCalls)
	}

	var output struct {
		DryRun    bool                    `json:"dryRun"`
		Operation string                  `json:"operation"`
		Input     linear.IssueCreateInput `json:"input"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode dry-run output: %v", err)
	}
	if !output.DryRun || output.Operation != "create" {
		t.Errorf("unexpected dry-run output: %+v", output)
	}
	if output.Input != (linear.IssueCreateInput{
		Title: "Example issue", Description: "body from stdin", TeamID: "team-uuid",
	}) {
		t.Errorf("unexpected dry-run input: %+v", output.Input)
	}
}

func TestHandleLinearUpdateDryRunDoesNotCallClient(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	client := &fakeLinearClient{}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearUpdate(&stdout, &stderr, bytes.NewReader(nil)),
		linearUpdateFlags(),
		[]string{"--clear-description", "--dry-run", "DEV-123"},
	)
	if err != nil {
		t.Fatalf("dry run returned error: %v", err)
	}
	if client.updateCalls != 0 {
		t.Errorf("expected no update calls, got %d", client.updateCalls)
	}

	var output struct {
		DryRun    bool   `json:"dryRun"`
		Operation string `json:"operation"`
		Input     struct {
			ID          string          `json:"id"`
			Description json.RawMessage `json:"description"`
		} `json:"input"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode dry-run output: %v", err)
	}
	if !output.DryRun || output.Operation != "update" ||
		output.Input.ID != "DEV-123" || string(output.Input.Description) != "null" {
		t.Errorf("unexpected dry-run output: %+v", output)
	}
}

func TestHandleLinearErrorsUseJSONEnvelope(t *testing.T) {
	tests := []struct {
		name       string
		setEnv     func(*testing.T)
		action     func(io.Writer, io.Writer) cli.ActionFunc
		flags      []cli.Flag
		args       []string
		client     linear.Clienter
		wantCode   string
		wantNoCall bool
	}{
		{
			name: "missing configuration",
			setEnv: func(t *testing.T) {
				t.Setenv("LINEAR_API_KEY", "")
				t.Setenv("LINEAR_TEAM_ID", "")
			},
			action: func(stdout, stderr io.Writer) cli.ActionFunc {
				return handleLinearCreate(stdout, stderr, bytes.NewReader(nil))
			},
			flags:      linearCreateFlags(),
			args:       []string{"--title", "Example issue"},
			client:     &fakeLinearClient{},
			wantCode:   "missing_configuration",
			wantNoCall: true,
		},
		{
			name: "invalid arguments",
			setEnv: func(t *testing.T) {
				t.Setenv("LINEAR_API_KEY", "token")
				t.Setenv("LINEAR_TEAM_ID", "team-uuid")
			},
			action: func(stdout, stderr io.Writer) cli.ActionFunc {
				return handleLinearCreate(stdout, stderr, bytes.NewReader(nil))
			},
			flags:      linearCreateFlags(),
			args:       []string{"--title", "Example", "--description", "one", "--description-file", "two"},
			client:     &fakeLinearClient{},
			wantCode:   "invalid_arguments",
			wantNoCall: true,
		},
		{
			name: "api failure",
			setEnv: func(t *testing.T) {
				t.Setenv("LINEAR_API_KEY", "token")
			},
			action: func(stdout, stderr io.Writer) cli.ActionFunc {
				return handleLinearGet(stdout, stderr)
			},
			args:     []string{"DEV-123"},
			client:   &fakeLinearClient{err: errors.New("Linear unavailable")},
			wantCode: "api_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setEnv(t)
			withLinearClient(t, tt.client)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err := runLinearCommand(t, tt.action(&stdout, &stderr), tt.flags, tt.args)
			if err == nil {
				t.Fatal("expected a non-zero exit error")
			}

			assertLinearErrorCode(t, stderr.Bytes(), tt.wantCode)
			if stdout.Len() != 0 {
				t.Errorf("expected no stdout, got %q", stdout.String())
			}
		})
	}
}

func TestLinearUsageErrorsUseJSONEnvelope(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := &cli.App{
		ExitErrHandler: func(_ *cli.Context, _ error) {},
		Commands: []*cli.Command{{
			Name: "linear",
			Subcommands: []*cli.Command{{
				Name:         "create",
				Flags:        linearCreateFlags(),
				OnUsageError: linearUsageError(&stderr),
				Action:       handleLinearCreate(&stdout, &stderr, bytes.NewReader(nil)),
			}},
		}},
	}
	err := app.Run([]string{"dev", "linear", "create", "--not-a-flag"})
	if err == nil {
		t.Fatal("expected a non-zero exit error")
	}
	assertLinearErrorCode(t, stderr.Bytes(), "invalid_arguments")
	if stdout.Len() != 0 {
		t.Errorf("expected no stdout, got %q", stdout.String())
	}
}

func runLinearCommand(t *testing.T, action cli.ActionFunc, flags []cli.Flag, args []string) error {
	t.Helper()
	app := &cli.App{
		ExitErrHandler: func(_ *cli.Context, _ error) {},
		Commands:       []*cli.Command{{Name: "linear", Action: action, Flags: flags}},
	}
	return app.Run(append([]string{"dev", "linear"}, args...))
}

func assertIssueOutput(t *testing.T, data []byte) {
	t.Helper()
	var output struct {
		ID          string `json:"id"`
		Identifier  string `json:"identifier"`
		URL         string `json:"url"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("decode issue output: %v", err)
	}
	if output.ID != "issue-uuid" || output.Identifier != "DEV-123" ||
		output.URL != "https://linear.app/example/issue/DEV-123/example" {
		t.Errorf("unexpected issue identity output: %+v", output)
	}
	if output.Title != "Example issue" || output.Description != "Example description" {
		t.Errorf("unexpected issue content output: %+v", output)
	}
}

func assertLinearErrorCode(t *testing.T, data []byte, wantCode string) {
	t.Helper()
	var output struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("decode stderr JSON: %v; stderr=%q", err, string(data))
	}
	if output.Error.Code != wantCode {
		t.Errorf("expected error code %q, got %q", wantCode, output.Error.Code)
	}
}
