package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/urfave/cli/v2"

	linear "github.com/thomasgormley/dev-cli-go/internal/linear"
)

type fakeLinearClient struct {
	createInput         linear.IssueCreateInput
	getID               string
	updateID            string
	updateInput         linear.UpdateIssueRequest
	issue               linear.Issue
	err                 error
	createCalls         int
	updateCalls         int
	teamPage            linear.TeamPage
	resolveTeam         linear.Team
	resolveErr          error
	listErr             error
	resolveTerm         string
	listRequest         linear.TeamListRequest
	projects            linear.ProjectPage
	projectErr          error
	projectTerm         string
	projectTeam         string
	projectPageRequest  linear.ProjectListRequest
	resolveProject      linear.Project
	resolveProjectErr   error
	labelPage           linear.LabelPage
	labelRequest        linear.LabelListRequest
	labelListTeam       string
	resolveLabels       map[string]linear.Label
	labelTeams          []string
	labelTerms          []string
	milestones          linear.ProjectMilestonePage
	milestoneErr        error
	milestoneProject    string
	milestoneTerm       string
	milestoneRequest    linear.ProjectMilestoneListRequest
	resolveMilestone    linear.ProjectMilestone
	resolveMilestoneErr error
	userPage            linear.UserPage
	userListErr         error
	userRequest         linear.UserListRequest
	resolveUser         linear.User
	resolveUserErr      error
	resolveUserTeamID   string
	resolveUserTerm     string
}

func (f *fakeLinearClient) CreateIssue(_ context.Context, input linear.IssueCreateInput) (linear.Issue, error) {
	f.createCalls++
	f.createInput = input
	return f.issue, f.err
}

func (f *fakeLinearClient) FindIssue(_ context.Context, id string) (linear.Issue, bool, error) {
	f.getID = id
	if f.err != nil {
		return linear.Issue{}, false, f.err
	}
	return f.issue, f.issue.ID != nil, nil
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

func (f *fakeLinearClient) ListTeams(_ context.Context, request linear.TeamListRequest) (linear.TeamPage, error) {
	f.listRequest = request
	if f.listErr != nil {
		return linear.TeamPage{}, f.listErr
	}
	return f.teamPage, nil
}

func (f *fakeLinearClient) ListLabels(
	_ context.Context,
	teamID string,
	request linear.LabelListRequest,
) (linear.LabelPage, error) {
	f.labelListTeam = teamID
	f.labelRequest = request
	if f.listErr != nil {
		return linear.LabelPage{}, f.listErr
	}
	return f.labelPage, nil
}

func (f *fakeLinearClient) FindTeam(_ context.Context, selector string) (linear.Team, bool, error) {
	f.resolveTerm = selector
	if f.resolveErr != nil {
		return linear.Team{}, false, f.resolveErr
	}
	return f.resolveTeam, f.resolveTeam.ID != nil, nil
}

func (f *fakeLinearClient) ListProjects(
	_ context.Context,
	teamID string,
	request linear.ProjectListRequest,
) (linear.ProjectPage, error) {
	f.projectTeam = teamID
	f.projectPageRequest = request
	if f.projectErr != nil {
		return linear.ProjectPage{}, f.projectErr
	}
	return f.projects, nil
}

func (f *fakeLinearClient) FindProject(_ context.Context, teamID string, selector string) (linear.Project, bool, error) {
	f.projectTeam = teamID
	f.projectTerm = selector
	if f.resolveProjectErr != nil {
		return linear.Project{}, false, f.resolveProjectErr
	}
	return f.resolveProject, f.resolveProject.ID != nil, nil
}

func (f *fakeLinearClient) ListUsers(
	_ context.Context,
	teamID string,
	request linear.UserListRequest,
) (linear.UserPage, error) {
	f.resolveUserTeamID = teamID
	f.userRequest = request
	if f.userListErr != nil {
		return linear.UserPage{}, f.userListErr
	}
	return f.userPage, nil
}

func (f *fakeLinearClient) FindAssignee(_ context.Context, teamID, selector string) (linear.User, bool, error) {
	f.resolveUserTeamID = teamID
	f.resolveUserTerm = selector
	if f.resolveUserErr != nil {
		return linear.User{}, false, f.resolveUserErr
	}
	return f.resolveUser, f.resolveUser.ID != nil, nil
}

func (f *fakeLinearClient) FindLabel(_ context.Context, teamID, selector string) (linear.Label, bool, error) {
	f.labelTeams = append(f.labelTeams, teamID)
	f.labelTerms = append(f.labelTerms, selector)
	label, ok := f.resolveLabels[selector]
	if !ok {
		return linear.Label{}, false, nil
	}
	return label, true, nil
}

func (f *fakeLinearClient) ListProjectMilestones(
	_ context.Context,
	projectID string,
	request linear.ProjectMilestoneListRequest,
) (linear.ProjectMilestonePage, error) {
	f.milestoneProject = projectID
	f.milestoneRequest = request
	if f.milestoneErr != nil {
		return linear.ProjectMilestonePage{}, f.milestoneErr
	}
	return f.milestones, nil
}

func (f *fakeLinearClient) FindProjectMilestone(
	_ context.Context,
	projectID string,
	selector string,
) (linear.ProjectMilestone, bool, error) {
	f.milestoneProject = projectID
	f.milestoneTerm = selector
	if f.resolveMilestoneErr != nil {
		return linear.ProjectMilestone{}, false, f.resolveMilestoneErr
	}
	return f.resolveMilestone, f.resolveMilestone.ID != nil, nil
}

func newLinearIssue() linear.Issue {
	return linear.Issue{
		ID:          "issue-uuid",
		Identifier:  "DEV-123",
		URL:         "https://linear.app/example/issue/DEV-123/example",
		Title:       "Example issue",
		Description: "Example description",
		Team:        linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"},
		Assignee: linear.User{
			ID: "user-uuid", Name: "Ada Lovelace", DisplayName: "Ada", Email: "ada@example.com", Active: true,
		},
		Priority: linearPriorityHigh,
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
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	client := &fakeLinearClient{issue: newLinearIssue(), resolveTeam: team}
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
	if !reflect.DeepEqual(client.createInput, linear.IssueCreateInput{
		Title: "Example issue", Description: "Example description", TeamID: "team-uuid",
	}) {
		t.Errorf("unexpected create input: %+v", client.createInput)
	}
	if client.resolveTerm != "team-uuid" {
		t.Errorf("expected LINEAR_TEAM_ID to be resolved, got %q", client.resolveTerm)
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
		!client.updateInput.SetDescription ||
		client.updateInput.Description != "from a file" {
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
	if !client.updateInput.ClearDescription || client.updateInput.SetDescription {
		t.Errorf("unexpected clear-description input: %+v", client.updateInput)
	}
}

func TestHandleLinearCreateDryRunDoesNotCallClient(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	t.Setenv("LINEAR_TEAM_ID", "team-uuid")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	client := &fakeLinearClient{resolveTeam: team}
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
		Resolved  struct {
			Team struct {
				ID   string `json:"id"`
				Key  string `json:"key"`
				Name string `json:"name"`
			} `json:"team"`
		} `json:"resolved"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode dry-run output: %v", err)
	}
	if !output.DryRun || output.Operation != "create" {
		t.Errorf("unexpected dry-run output: %+v", output)
	}
	if !reflect.DeepEqual(output.Input, linear.IssueCreateInput{
		Title: "Example issue", Description: "body from stdin", TeamID: "team-uuid",
	}) {
		t.Errorf("unexpected dry-run input: %+v", output.Input)
	}
	if output.Resolved.Team.ID != "team-uuid" || output.Resolved.Team.Key != "DEV" ||
		output.Resolved.Team.Name != "Development" {
		t.Errorf("unexpected resolved team: %+v", output.Resolved.Team)
	}
}

func TestHandleLinearCreateResolvesTeamFlagBeforeMutation(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	t.Setenv("LINEAR_TEAM_ID", "env-team")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	issue := newLinearIssue()
	issue.Team = team
	client := &fakeLinearClient{issue: issue, resolveTeam: team}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearCreate(&stdout, &stderr, bytes.NewReader(nil)),
		linearCreateFlags(),
		[]string{"--title", "Example issue", "--team", "DEV"},
	)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}
	if client.resolveTerm != "DEV" {
		t.Errorf("expected --team to take precedence, got %q", client.resolveTerm)
	}
	if client.createInput.TeamID != "team-uuid" {
		t.Errorf("expected resolved team selector, got %q", client.createInput.TeamID)
	}

	var output struct {
		Team struct {
			ID   string `json:"id"`
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"team"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode create output: %v", err)
	}
	if output.Team.ID != "team-uuid" || output.Team.Key != "DEV" || output.Team.Name != "Development" {
		t.Errorf("unexpected output team: %+v", output.Team)
	}
}

func TestHandleLinearCreateResolvesProjectBeforeMutation(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	project := linear.Project{ID: "project-uuid", Name: "Agent work", SlugID: "agent-work"}
	issue := newLinearIssue()
	issue.Team = team
	issue.Project = project
	client := &fakeLinearClient{issue: issue, resolveTeam: team, resolveProject: project}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearCreate(&stdout, &stderr, bytes.NewReader(nil)),
		linearCreateFlags(),
		[]string{"--title", "Example issue", "--team", "DEV", "--project", "agent-work"},
	)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}
	if client.createInput.ProjectID != "project-uuid" || client.projectTeam != "team-uuid" ||
		client.projectTerm != "agent-work" {
		t.Errorf(
			"unexpected project resolution: input=%+v team=%q selector=%q",
			client.createInput,
			client.projectTeam,
			client.projectTerm,
		)
	}

	var output struct {
		Project linearProjectOutput `json:"project"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode create output: %v", err)
	}
	if output.Project.ID != "project-uuid" || output.Project.SlugID != "agent-work" {
		t.Errorf("unexpected project output: %+v", output.Project)
	}
}

func TestHandleLinearCreateRequiresProjectForMilestone(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	client := &fakeLinearClient{}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearCreate(&stdout, &stderr, bytes.NewReader(nil)),
		linearCreateFlags(),
		[]string{"--title", "Example issue", "--milestone", "Launch"},
	)
	if err == nil {
		t.Fatal("expected a non-zero exit error")
	}
	assertLinearErrorCode(t, stderr.Bytes(), "invalid_arguments")
	if client.createCalls != 0 || client.milestoneTerm != "" {
		t.Errorf("expected no resolution or mutation, got milestones=%q creates=%d", client.milestoneTerm, client.createCalls)
	}
}

func TestHandleLinearCreateResolvesProjectMilestoneBeforeMutation(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	project := linear.Project{ID: "project-uuid", Name: "Agent work", SlugID: "agent-work"}
	milestone := linear.ProjectMilestone{ID: "milestone-uuid", Name: "Launch", Project: project}
	issue := newLinearIssue()
	issue.Team = team
	issue.Project = project
	issue.ProjectMilestone = milestone
	client := &fakeLinearClient{
		issue: issue, resolveTeam: team, resolveProject: project, resolveMilestone: milestone,
	}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearCreate(&stdout, &stderr, bytes.NewReader(nil)),
		linearCreateFlags(),
		[]string{"--title", "Example issue", "--team", "DEV", "--project", "agent-work", "--milestone", "Launch"},
	)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}
	if client.createInput.ProjectID != "project-uuid" || client.createInput.ProjectMilestoneID != "milestone-uuid" ||
		client.milestoneProject != "project-uuid" || client.milestoneTerm != "Launch" {
		t.Errorf(
			"unexpected milestone resolution: input=%+v project=%q selector=%q",
			client.createInput,
			client.milestoneProject,
			client.milestoneTerm,
		)
	}
}

func TestHandleLinearTeamList(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	client := &fakeLinearClient{teamPage: linear.TeamPage{
		Items:    []linear.Team{{ID: "team-uuid", Key: "DEV", Name: "Development"}},
		PageInfo: linear.PageInfo{HasNextPage: true, NextCursor: "next-cursor"},
	}}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearTeamList(&stdout, &stderr),
		linearTeamListFlags(),
		[]string{"--limit", "25", "--cursor", "previous-cursor"},
	)
	if err != nil {
		t.Fatalf("list returned error: %v", err)
	}
	if client.listRequest != (linear.TeamListRequest{Limit: 25, Cursor: "previous-cursor"}) {
		t.Errorf("unexpected list request: %+v", client.listRequest)
	}

	var output struct {
		Items []struct {
			ID   string `json:"id"`
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"items"`
		PageInfo struct {
			HasNextPage bool   `json:"hasNextPage"`
			NextCursor  string `json:"nextCursor"`
		} `json:"pageInfo"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode list output: %v", err)
	}
	if len(output.Items) != 1 || output.Items[0].ID != "team-uuid" ||
		output.Items[0].Key != "DEV" || output.Items[0].Name != "Development" {
		t.Errorf("unexpected list items: %+v", output.Items)
	}
	if !output.PageInfo.HasNextPage || output.PageInfo.NextCursor != "next-cursor" {
		t.Errorf("unexpected page info: %+v", output.PageInfo)
	}
}

func TestHandleLinearUserList(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	client := &fakeLinearClient{
		resolveTeam: team,
		userPage: linear.UserPage{
			Items: []linear.User{{
				ID: "user-uuid", Name: "Ada Lovelace", DisplayName: "Ada", Email: "ada@example.com", Active: true,
			}},
			PageInfo: linear.PageInfo{HasNextPage: true, NextCursor: "next-cursor"},
		},
	}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearUserList(&stdout, &stderr),
		linearUserListFlags(),
		[]string{"--team", "DEV", "--limit", "25", "--cursor", "previous-cursor"},
	)
	if err != nil {
		t.Fatalf("list returned error: %v", err)
	}
	if client.resolveTerm != "DEV" || client.resolveUserTeamID != "team-uuid" ||
		client.userRequest != (linear.UserListRequest{Limit: 25, Cursor: "previous-cursor"}) {
		t.Errorf("unexpected user listing calls: team=%q teamID=%q request=%+v", client.resolveTerm,
			client.resolveUserTeamID, client.userRequest)
	}

	var output struct {
		Items []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
			Email       string `json:"email"`
		} `json:"items"`
		PageInfo struct {
			HasNextPage bool   `json:"hasNextPage"`
			NextCursor  string `json:"nextCursor"`
		} `json:"pageInfo"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode user list output: %v", err)
	}
	if len(output.Items) != 1 || output.Items[0].ID != "user-uuid" ||
		output.Items[0].DisplayName != "Ada" || output.Items[0].Email != "ada@example.com" {
		t.Errorf("unexpected user list output: %+v", output.Items)
	}
	if !output.PageInfo.HasNextPage || output.PageInfo.NextCursor != "next-cursor" {
		t.Errorf("unexpected page info: %+v", output.PageInfo)
	}
}

func TestHandleLinearProjectList(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	client := &fakeLinearClient{
		resolveTeam: team,
		projects: linear.ProjectPage{
			Items:    []linear.Project{{ID: "project-uuid", Name: "Agent work", SlugID: "agent-work"}},
			PageInfo: linear.PageInfo{HasNextPage: true, NextCursor: "next-cursor"},
		},
	}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearProjectList(&stdout, &stderr),
		linearProjectListFlags(),
		[]string{"--team", "DEV", "--limit", "25", "--cursor", "previous-cursor"},
	)
	if err != nil {
		t.Fatalf("list returned error: %v", err)
	}
	if client.projectTeam != "team-uuid" ||
		client.projectPageRequest != (linear.ProjectListRequest{Limit: 25, Cursor: "previous-cursor"}) {
		t.Errorf("unexpected list request: team=%q request=%+v", client.projectTeam, client.projectPageRequest)
	}

	var output struct {
		Items    []linearProjectOutput `json:"items"`
		PageInfo struct {
			HasNextPage bool   `json:"hasNextPage"`
			NextCursor  string `json:"nextCursor"`
		} `json:"pageInfo"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode list output: %v", err)
	}
	if len(output.Items) != 1 || output.Items[0].ID != "project-uuid" ||
		output.Items[0].SlugID != "agent-work" {
		t.Errorf("unexpected list output: %+v", output)
	}
	if !output.PageInfo.HasNextPage || output.PageInfo.NextCursor != "next-cursor" {
		t.Errorf("unexpected page info: %+v", output.PageInfo)
	}
}

func TestHandleLinearLabelListIncludesTeamWorkspaceAndGroups(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	client := &fakeLinearClient{resolveTeam: team, labelPage: linear.LabelPage{
		Items: []linear.Label{
			{ID: "team-label", Name: "Bug", Team: team},
			{ID: "workspace-group", Name: "Type", IsGroup: true},
		},
		PageInfo: linear.PageInfo{HasNextPage: true, NextCursor: "next-cursor"},
	}}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(t, handleLinearLabelList(&stdout, &stderr), linearLabelListFlags(),
		[]string{"--team", "DEV", "--limit", "25", "--cursor", "previous-cursor"})
	if err != nil {
		t.Fatalf("list returned error: %v", err)
	}
	if client.resolveTerm != "DEV" || client.labelListTeam != "team-uuid" ||
		client.labelRequest != (linear.LabelListRequest{Limit: 25, Cursor: "previous-cursor"}) {
		t.Errorf(
			"unexpected label list request: selector=%q team=%q request=%+v",
			client.resolveTerm,
			client.labelListTeam,
			client.labelRequest,
		)
	}
}

func TestHandleLinearProjectMilestoneList(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	project := linear.Project{ID: "project-uuid", Name: "Agent work", SlugID: "agent-work"}
	client := &fakeLinearClient{resolveTeam: team, resolveProject: project, milestones: linear.ProjectMilestonePage{
		Items:    []linear.ProjectMilestone{{ID: "milestone-uuid", Name: "Launch", Project: project}},
		PageInfo: linear.PageInfo{HasNextPage: true, NextCursor: "next-cursor"},
	}}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(t, handleLinearProjectMilestoneList(&stdout, &stderr), linearProjectMilestoneListFlags(),
		[]string{"--team", "DEV", "--project", "agent-work", "--limit", "25", "--cursor", "previous-cursor"})
	if err != nil {
		t.Fatalf("list returned error: %v", err)
	}
	if client.milestoneProject != "project-uuid" ||
		client.milestoneRequest != (linear.ProjectMilestoneListRequest{Limit: 25, Cursor: "previous-cursor"}) {
		t.Errorf(
			"unexpected milestone list request: project=%q request=%+v",
			client.milestoneProject,
			client.milestoneRequest,
		)
	}
	var output struct {
		Items    []linearProjectMilestoneOutput `json:"items"`
		PageInfo linearPageInfoOutput           `json:"pageInfo"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode list output: %v", err)
	}
	if len(output.Items) != 1 || output.Items[0].ID != "milestone-uuid" || output.Items[0].Name != "Launch" ||
		!output.PageInfo.HasNextPage || output.PageInfo.NextCursor != "next-cursor" {
		t.Errorf("unexpected milestone list output: %+v", output)
	}
}

func TestHandleLinearUpdateResolvesAndClearsProject(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	project := linear.Project{ID: "project-uuid", Name: "Agent work", SlugID: "agent-work"}
	issue := newLinearIssue()
	issue.Team = team
	issue.Project = project
	client := &fakeLinearClient{issue: issue, resolveProject: project}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearUpdate(&stdout, &stderr, bytes.NewReader(nil)),
		linearUpdateFlags(),
		[]string{"--project", "agent-work", "DEV-123"},
	)
	if err != nil {
		t.Fatalf("update returned error: %v", err)
	}
	if !client.updateInput.SetProject || client.updateInput.ProjectID != "project-uuid" ||
		client.updateInput.ClearProject || client.projectTeam != "team-uuid" || client.projectTerm != "agent-work" {
		t.Errorf(
			"unexpected project update: input=%+v team=%q selector=%q",
			client.updateInput,
			client.projectTeam,
			client.projectTerm,
		)
	}
	var movedOutput struct {
		Project *linearProjectOutput `json:"project"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &movedOutput); err != nil {
		t.Fatalf("decode moved issue output: %v", err)
	}
	if movedOutput.Project == nil || movedOutput.Project.ID != "project-uuid" {
		t.Errorf("expected moved issue output to report project, got %+v", movedOutput)
	}

	stdout.Reset()
	stderr.Reset()
	client.issue.Project = linear.Project{}
	err = runLinearCommand(
		t,
		handleLinearUpdate(&stdout, &stderr, bytes.NewReader(nil)),
		linearUpdateFlags(),
		[]string{"--clear-project", "DEV-123"},
	)
	if err != nil {
		t.Fatalf("clear project returned error: %v", err)
	}
	if !client.updateInput.ClearProject || client.updateInput.SetProject {
		t.Errorf("unexpected clear-project input: %+v", client.updateInput)
	}
	var clearedOutput struct {
		Project *linearProjectOutput `json:"project"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &clearedOutput); err != nil {
		t.Fatalf("decode cleared issue output: %v", err)
	}
	if clearedOutput.Project != nil {
		t.Errorf("expected cleared issue output to report no project, got %+v", clearedOutput)
	}
}

func TestHandleLinearUpdateProjectMilestones(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	oldProject := linear.Project{ID: "project-old", Name: "Old", SlugID: "old"}
	newProject := linear.Project{ID: "project-new", Name: "New", SlugID: "new"}
	oldMilestone := linear.ProjectMilestone{ID: "milestone-old", Name: "Old launch", Project: oldProject}
	newMilestone := linear.ProjectMilestone{ID: "milestone-new", Name: "New launch", Project: newProject}

	tests := []struct {
		name                 string
		args                 []string
		project              linear.Project
		milestone            linear.ProjectMilestone
		resolveProject       linear.Project
		resolveMilestone     linear.ProjectMilestone
		wantError            bool
		wantProjectID        string
		wantMilestoneID      string
		wantClearProject     bool
		wantClearMilestone   bool
		wantMilestoneProject string
	}{
		{
			name:    "infers current project for milestone",
			args:    []string{"--milestone", "Old launch", "DEV-123"},
			project: oldProject, milestone: oldMilestone, resolveMilestone: oldMilestone,
			wantMilestoneID: "milestone-old", wantMilestoneProject: "project-old",
		},
		{
			name:    "rejects project move with existing milestone",
			args:    []string{"--project", "new", "DEV-123"},
			project: oldProject, milestone: oldMilestone, resolveProject: newProject, wantError: true,
		},
		{
			name:    "allows project move when clearing milestone",
			args:    []string{"--project", "new", "--clear-milestone", "DEV-123"},
			project: oldProject, milestone: oldMilestone, resolveProject: newProject,
			wantProjectID: "project-new", wantClearMilestone: true,
		},
		{
			name:    "allows combined project and milestone move",
			args:    []string{"--project", "new", "--milestone", "New launch", "DEV-123"},
			project: oldProject, milestone: oldMilestone, resolveProject: newProject, resolveMilestone: newMilestone,
			wantProjectID: "project-new", wantMilestoneID: "milestone-new", wantMilestoneProject: "project-new",
		},
		{
			name:    "clearing project clears milestone relationship",
			args:    []string{"--clear-project", "DEV-123"},
			project: oldProject, milestone: oldMilestone, wantClearProject: true, wantClearMilestone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := newLinearIssue()
			issue.Team = team
			issue.Project = tt.project
			issue.ProjectMilestone = tt.milestone
			client := &fakeLinearClient{
				issue: issue, resolveProject: tt.resolveProject, resolveMilestone: tt.resolveMilestone,
			}
			withLinearClient(t, client)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err := runLinearCommand(
				t,
				handleLinearUpdate(&stdout, &stderr, bytes.NewReader(nil)),
				linearUpdateFlags(),
				tt.args,
			)
			if tt.wantError {
				if err == nil {
					t.Fatal("expected a non-zero exit error")
				}
				assertLinearErrorCode(t, stderr.Bytes(), "invalid_arguments")
				if client.updateCalls != 0 {
					t.Errorf("expected no mutation, got %d update calls", client.updateCalls)
				}
				return
			}
			if err != nil {
				t.Fatalf("update returned error: %v", err)
			}
			if tt.wantProjectID != "" &&
				(!client.updateInput.SetProject || client.updateInput.ProjectID != tt.wantProjectID) {
				t.Errorf("unexpected project input: %+v", client.updateInput)
			}
			if tt.wantMilestoneID != "" &&
				(!client.updateInput.SetProjectMilestone ||
					client.updateInput.ProjectMilestoneID != tt.wantMilestoneID) {
				t.Errorf("unexpected milestone input: %+v", client.updateInput)
			}
			if client.updateInput.ClearProject != tt.wantClearProject ||
				client.updateInput.ClearProjectMilestone != tt.wantClearMilestone ||
				client.milestoneProject != tt.wantMilestoneProject {
				t.Errorf(
					"unexpected relationship update: input=%+v milestoneProject=%q",
					client.updateInput,
					client.milestoneProject,
				)
			}
		})
	}
}

func TestHandleLinearUpdateClearProjectDryRunReportsMilestoneClear(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	client := &fakeLinearClient{}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearUpdate(&stdout, &stderr, bytes.NewReader(nil)),
		linearUpdateFlags(),
		[]string{"--clear-project", "--dry-run", "DEV-123"},
	)
	if err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}
	if client.getID != "" || client.updateCalls != 0 {
		t.Errorf("expected no issue read or mutation, got get=%q updates=%d", client.getID, client.updateCalls)
	}
	var output struct {
		DryRun bool `json:"dryRun"`
		Input  struct {
			ProjectID          any `json:"projectId"`
			ProjectMilestoneID any `json:"projectMilestoneId"`
		} `json:"input"`
		Resolved struct {
			Project          *linearProjectOutput          `json:"project"`
			ProjectMilestone *linearProjectMilestoneOutput `json:"projectMilestone"`
		} `json:"resolved"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode dry-run output: %v", err)
	}
	if !output.DryRun || output.Input.ProjectID != nil || output.Input.ProjectMilestoneID != nil ||
		output.Resolved.Project != nil || output.Resolved.ProjectMilestone != nil {
		t.Errorf("unexpected clear-project dry-run output: %+v", output)
	}
}

func TestHandleLinearUpdateProjectDryRunResolvesWithoutMutation(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	project := linear.Project{ID: "project-uuid", Name: "Agent work", SlugID: "agent-work"}
	issue := newLinearIssue()
	issue.Team = team
	client := &fakeLinearClient{issue: issue, resolveProject: project}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearUpdate(&stdout, &stderr, bytes.NewReader(nil)),
		linearUpdateFlags(),
		[]string{"--project", "agent-work", "--dry-run", "DEV-123"},
	)
	if err != nil {
		t.Fatalf("dry run returned error: %v", err)
	}
	if client.updateCalls != 0 || client.projectTeam != "team-uuid" || client.projectTerm != "agent-work" {
		t.Errorf(
			"expected resolution but no mutation, got calls=%d team=%q selector=%q",
			client.updateCalls,
			client.projectTeam,
			client.projectTerm,
		)
	}

	var output struct {
		DryRun bool `json:"dryRun"`
		Input  struct {
			ProjectID string `json:"projectId"`
		} `json:"input"`
		Resolved struct {
			Project linearProjectOutput `json:"project"`
		} `json:"resolved"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode dry-run output: %v", err)
	}
	if !output.DryRun || output.Input.ProjectID != "project-uuid" || output.Resolved.Project.ID != "project-uuid" {
		t.Errorf("unexpected dry-run output: %+v", output)
	}
}

func TestHandleLinearUpdateRejectsConflictingProjectFlags(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	client := &fakeLinearClient{}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearUpdate(&stdout, &stderr, bytes.NewReader(nil)),
		linearUpdateFlags(),
		[]string{"--project", "agent-work", "--clear-project", "DEV-123"},
	)
	if err == nil {
		t.Fatal("expected a non-zero exit error")
	}
	assertLinearErrorCode(t, stderr.Bytes(), "invalid_arguments")
	if client.getID != "" || client.updateCalls != 0 {
		t.Errorf("expected no issue read or mutation, got get=%q updates=%d", client.getID, client.updateCalls)
	}
}

func TestHandleLinearCreateReportsProjectResolutionErrors(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	client := &fakeLinearClient{
		resolveTeam:       team,
		resolveProjectErr: errors.New("multiple active projects match \"agent\""),
	}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearCreate(&stdout, &stderr, bytes.NewReader(nil)),
		linearCreateFlags(),
		[]string{"--title", "Example issue", "--team", "DEV", "--project", "agent"},
	)
	if err == nil {
		t.Fatal("expected a non-zero exit error")
	}
	assertLinearErrorCode(t, stderr.Bytes(), "ambiguous_project")
	if client.createCalls != 0 {
		t.Errorf("expected no mutation calls, got %d", client.createCalls)
	}
}

func TestHandleLinearCreateResolvesAssigneeAndPriority(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	user := linear.User{ID: "user-uuid", Name: "Ada Lovelace", DisplayName: "Ada", Email: "ada@example.com", Active: true}
	client := &fakeLinearClient{issue: newLinearIssue(), resolveTeam: team, resolveUser: user}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearCreate(&stdout, &stderr, bytes.NewReader(nil)),
		linearCreateFlags(),
		[]string{"--title", "Example issue", "--team", "DEV", "--assignee", "me", "--priority", "HIGH"},
	)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}
	if client.resolveUserTeamID != "team-uuid" || client.resolveUserTerm != "me" ||
		client.createInput.AssigneeID != "user-uuid" || client.createInput.Priority != linearPriorityHigh {
		t.Errorf("unexpected create input: %+v, user selector=%q team=%q", client.createInput,
			client.resolveUserTerm, client.resolveUserTeamID)
	}

	var output struct {
		Assignee *struct {
			ID string `json:"id"`
		} `json:"assignee"`
		Priority string `json:"priority"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode create output: %v", err)
	}
	if output.Assignee == nil || output.Assignee.ID != "user-uuid" || output.Priority != "high" {
		t.Errorf("unexpected created issue output: %+v", output)
	}
}

func TestHandleLinearCreateDryRunReportsResolvedAssigneeAndPriority(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	user := linear.User{
		ID: "user-uuid", Name: "Ada Lovelace", DisplayName: "Ada", Email: "ada@example.com", Active: true,
	}
	client := &fakeLinearClient{resolveTeam: team, resolveUser: user}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearCreate(&stdout, &stderr, bytes.NewReader(nil)),
		linearCreateFlags(),
		[]string{
			"--title", "Example issue", "--team", "DEV", "--assignee", "ada@example.com",
			"--priority", "urgent", "--dry-run",
		},
	)
	if err != nil {
		t.Fatalf("dry run returned error: %v", err)
	}
	if client.createCalls != 0 {
		t.Errorf("expected no create calls, got %d", client.createCalls)
	}

	var output struct {
		Input struct {
			AssigneeID string `json:"assigneeId"`
			Priority   int    `json:"priority"`
		} `json:"input"`
		Resolved struct {
			Assignee struct {
				ID          string `json:"id"`
				DisplayName string `json:"displayName"`
			} `json:"assignee"`
			Priority string `json:"priority"`
		} `json:"resolved"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode dry-run output: %v", err)
	}
	if output.Input.AssigneeID != "user-uuid" || output.Input.Priority != linearPriorityUrgent ||
		output.Resolved.Assignee.ID != "user-uuid" || output.Resolved.Assignee.DisplayName != "Ada" ||
		output.Resolved.Priority != "urgent" {
		t.Errorf("unexpected dry-run resolution: %+v", output)
	}
}

func TestHandleLinearUpdateSupportsCombinedAssigneeAndPriorityMutations(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	user := linear.User{ID: "user-uuid", Name: "Ada Lovelace", DisplayName: "Ada", Email: "ada@example.com", Active: true}
	client := &fakeLinearClient{issue: newLinearIssue(), resolveUser: user}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearUpdate(&stdout, &stderr, bytes.NewReader(nil)),
		linearUpdateFlags(),
		[]string{"--assignee", "ada@example.com", "--priority", "low", "DEV-123"},
	)
	if err != nil {
		t.Fatalf("update returned error: %v", err)
	}
	if client.getID != "DEV-123" || client.resolveUserTeamID != "team-uuid" ||
		!client.updateInput.SetAssignee || client.updateInput.AssigneeID != "user-uuid" ||
		!client.updateInput.SetPriority || client.updateInput.Priority != linearPriorityLow {
		t.Errorf("unexpected combined update: get=%q input=%+v", client.getID, client.updateInput)
	}
}

func TestHandleLinearUpdateClearsAssigneeAndSetsNoPriority(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	client := &fakeLinearClient{issue: newLinearIssue()}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearUpdate(&stdout, &stderr, bytes.NewReader(nil)),
		linearUpdateFlags(),
		[]string{"--clear-assignee", "--priority", "none", "DEV-123"},
	)
	if err != nil {
		t.Fatalf("update returned error: %v", err)
	}
	if !client.updateInput.ClearAssignee || client.updateInput.SetAssignee ||
		!client.updateInput.SetPriority || client.updateInput.Priority != linearPriorityNone {
		t.Errorf("unexpected clear update input: %+v", client.updateInput)
	}
	if client.getID != "" || client.resolveUserTerm != "" {
		t.Errorf("clear-assignee should not need a user lookup: get=%q selector=%q", client.getID, client.resolveUserTerm)
	}
}

func TestHandleLinearUpdateRejectsInvalidPriorityAndConflictingAssigneeFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "invalid priority", args: []string{"--priority", "critical", "DEV-123"}},
		{name: "empty priority", args: []string{"--priority", "", "DEV-123"}},
		{name: "clear and set assignee", args: []string{"--assignee", "me", "--clear-assignee", "DEV-123"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LINEAR_API_KEY", "token")
			client := &fakeLinearClient{}
			withLinearClient(t, client)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err := runLinearCommand(t, handleLinearUpdate(&stdout, &stderr, bytes.NewReader(nil)), linearUpdateFlags(), tt.args)
			if err == nil {
				t.Fatal("expected an invalid arguments error")
			}
			assertLinearErrorCode(t, stderr.Bytes(), "invalid_arguments")
			if client.updateCalls != 0 || client.getID != "" {
				t.Errorf("expected no client calls, got update=%d get=%q", client.updateCalls, client.getID)
			}
		})
	}
}

func TestHandleLinearCreateReportsAssigneeResolutionErrors(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	client := &fakeLinearClient{
		resolveTeam:    team,
		resolveUserErr: errors.New("multiple active team members match \"Alex\""),
	}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearCreate(&stdout, &stderr, bytes.NewReader(nil)),
		linearCreateFlags(),
		[]string{"--title", "Example issue", "--team", "DEV", "--assignee", "Alex"},
	)
	if err == nil {
		t.Fatal("expected an assignee resolution error")
	}
	assertLinearErrorCode(t, stderr.Bytes(), "ambiguous_assignee")
	if client.createCalls != 0 {
		t.Errorf("expected no mutation calls, got %d", client.createCalls)
	}

}

func TestLinearPriorityNames(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  int
	}{
		{name: "none", value: "none", want: linearPriorityNone},
		{name: "urgent", value: "urgent", want: linearPriorityUrgent},
		{name: "high", value: "high", want: linearPriorityHigh},
		{name: "medium", value: "medium", want: linearPriorityMedium},
		{name: "low", value: "low", want: linearPriorityLow},
		{name: "case insensitive", value: "HIGH", want: linearPriorityHigh},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := linearPriority(tt.value)
			if err != nil {
				t.Fatalf("parse priority: %v", err)
			}
			if got != tt.want {
				t.Errorf("expected priority %d, got %d", tt.want, got)
			}
		})
	}
}

func TestHandleLinearCreateReportsTeamResolutionErrors(t *testing.T) {
	tests := []struct {
		name        string
		resolveErr  error
		resolveTeam linear.Team
		wantCode    string
	}{
		{
			name:     "not found",
			wantCode: "team_not_found",
		},
		{
			name:       "ambiguous",
			resolveErr: errors.New("multiple active teams match \"platform\""),
			wantCode:   "ambiguous_team",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LINEAR_API_KEY", "token")
			client := &fakeLinearClient{resolveErr: tt.resolveErr, resolveTeam: tt.resolveTeam}
			withLinearClient(t, client)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err := runLinearCommand(
				t,
				handleLinearCreate(&stdout, &stderr, bytes.NewReader(nil)),
				linearCreateFlags(),
				[]string{"--title", "Example issue", "--team", "missing"},
			)
			if err == nil {
				t.Fatal("expected a non-zero exit error")
			}
			assertLinearErrorCode(t, stderr.Bytes(), tt.wantCode)
			if client.createCalls != 0 {
				t.Errorf("expected no mutation calls, got %d", client.createCalls)
			}
		})
	}
}

func TestHandleLinearCreateRequiresTeamSelector(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	t.Setenv("LINEAR_TEAM_ID", "")
	client := &fakeLinearClient{}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearCreate(&stdout, &stderr, bytes.NewReader(nil)),
		linearCreateFlags(),
		[]string{"--title", "Example issue"},
	)
	if err == nil {
		t.Fatal("expected a non-zero exit error")
	}
	assertLinearErrorCode(t, stderr.Bytes(), "missing_configuration")
	if client.resolveTerm != "" || client.createCalls != 0 {
		t.Errorf("expected no resolution or mutation, got selector=%q calls=%d", client.resolveTerm, client.createCalls)
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

func assertComposedLinearIssueOutput(t *testing.T, data []byte) {
	t.Helper()
	var output struct {
		ID               string                        `json:"id"`
		Identifier       string                        `json:"identifier"`
		URL              string                        `json:"url"`
		Title            string                        `json:"title"`
		Description      string                        `json:"description"`
		Team             linearTeamOutput              `json:"team"`
		Assignee         *linearUserOutput             `json:"assignee"`
		Priority         string                        `json:"priority"`
		Project          *linearProjectOutput          `json:"project"`
		ProjectMilestone *linearProjectMilestoneOutput `json:"projectMilestone"`
		Labels           []linearLabelOutput           `json:"labels"`
	}
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("decode issue output: %v", err)
	}
	if output.ID != "issue-uuid" || output.Identifier != "DEV-123" ||
		output.URL != "https://linear.app/example/issue/DEV-123/example" ||
		output.Title != "Example issue" || output.Description != "Example description" {
		t.Errorf("unexpected issue identity and content: %+v", output)
	}
	if output.Team.ID != "team-uuid" || output.Assignee == nil || output.Assignee.ID != "user-uuid" ||
		output.Priority != "high" || output.Project == nil || output.Project.ID != "project-uuid" ||
		output.ProjectMilestone == nil || output.ProjectMilestone.ID != "milestone-uuid" {
		t.Errorf("unexpected composed issue properties: %+v", output)
	}
	if got, want := output.Labels, []linearLabelOutput{
		{ID: "label-bug", Name: "Bug", Team: &linearTeamOutput{ID: "team-uuid", Key: "DEV", Name: "Development"}},
		{ID: "label-platform", Name: "Platform", Team: nil},
	}; !reflect.DeepEqual(got, want) {
		t.Errorf("unexpected labels: got %+v, want %+v", got, want)
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

func TestHandleLinearCreateResolvesMultipleLabelsBeforeMutation(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	t.Setenv("LINEAR_TEAM_ID", "team-uuid")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	client := &fakeLinearClient{
		issue:       newLinearIssue(),
		resolveTeam: team,
		resolveLabels: map[string]linear.Label{
			"Bug":      {ID: "label-bug", Name: "Bug", Team: team},
			"Platform": {ID: "label-platform", Name: "Platform"},
		},
	}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearCreate(&stdout, &stderr, bytes.NewReader(nil)),
		linearCreateFlags(),
		[]string{"--title", "Example issue", "--label", "Bug", "--label", "Platform"},
	)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}
	if got, want := client.createInput.LabelIDs, []string{"label-bug", "label-platform"}; !slices.Equal(got, want) {
		t.Errorf("expected resolved labels %v, got %v", want, got)
	}
	if got, want := client.labelTeams, []string{"team-uuid", "team-uuid"}; !slices.Equal(got, want) {
		t.Errorf("expected labels resolved within team %v, got %v", want, got)
	}
}

func TestLinearCommandsKeepComposedIssueShape(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	project := linear.Project{ID: "project-uuid", Name: "Agent work", SlugID: "agent-work"}
	milestone := linear.ProjectMilestone{ID: "milestone-uuid", Name: "Launch", Project: project}
	assignee := linear.User{ID: "user-uuid", Name: "Ada Lovelace", DisplayName: "Ada", Email: "ada@example.com", Active: true}
	bug := linear.Label{ID: "label-bug", Name: "Bug", Team: team}
	platform := linear.Label{ID: "label-platform", Name: "Platform"}
	issue := newLinearIssue()
	issue.Team = team
	issue.Project = project
	issue.ProjectMilestone = milestone
	issue.Assignee = assignee
	issue.Labels = linear.LabelConnection{Nodes: []linear.Label{bug, platform}}
	client := &fakeLinearClient{
		issue:            issue,
		resolveTeam:      team,
		resolveProject:   project,
		resolveMilestone: milestone,
		resolveUser:      assignee,
		resolveLabels: map[string]linear.Label{
			"Bug": bug, "Platform": platform,
		},
	}
	withLinearClient(t, client)

	tests := []struct {
		name  string
		flags []cli.Flag
		args  []string
	}{
		{
			name:  "create",
			flags: linearCreateFlags(),
			args: []string{
				"--title", "Example issue", "--description", "Example description", "--team", "DEV",
				"--assignee", "ada@example.com", "--priority", "high", "--project", "agent-work",
				"--milestone", "Launch", "--label", "Bug", "--label", "Platform",
			},
		},
		{
			name: "get",
			args: []string{"DEV-123"},
		},
		{
			name:  "update",
			flags: linearUpdateFlags(),
			args: []string{
				"--title", "Updated issue", "--assignee", "ada@example.com", "--priority", "high",
				"--project", "agent-work", "--milestone", "Launch", "--add-label", "Bug", "DEV-123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			var action cli.ActionFunc
			switch tt.name {
			case "create":
				action = handleLinearCreate(&stdout, &stderr, bytes.NewReader(nil))
			case "update":
				action = handleLinearUpdate(&stdout, &stderr, bytes.NewReader(nil))
			default:
				action = handleLinearGet(&stdout, &stderr)
			}
			err := runLinearCommand(t, action, tt.flags, tt.args)
			if err != nil {
				t.Fatalf("command returned error: %v; stderr=%s", err, stderr.String())
			}
			assertComposedLinearIssueOutput(t, stdout.Bytes())
		})
	}

	if got, want := client.createInput.LabelIDs, []string{"label-bug", "label-platform"}; !slices.Equal(got, want) {
		t.Errorf("expected resolved create labels %v, got %v", want, got)
	}
	if !client.updateInput.SetAssignee || client.updateInput.AssigneeID != "user-uuid" ||
		!client.updateInput.SetProject || client.updateInput.ProjectID != "project-uuid" ||
		!client.updateInput.SetProjectMilestone || client.updateInput.ProjectMilestoneID != "milestone-uuid" {
		t.Errorf("unexpected composed update input: %+v", client.updateInput)
	}
}

func TestHandleLinearCreateDryRunComposesAllIssueProperties(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	project := linear.Project{ID: "project-uuid", Name: "Agent work", SlugID: "agent-work"}
	milestone := linear.ProjectMilestone{ID: "milestone-uuid", Name: "Launch", Project: project}
	assignee := linear.User{ID: "user-uuid", Name: "Ada Lovelace", DisplayName: "Ada", Email: "ada@example.com", Active: true}
	bug := linear.Label{ID: "label-bug", Name: "Bug", Team: team}
	platform := linear.Label{ID: "label-platform", Name: "Platform"}
	client := &fakeLinearClient{
		resolveTeam:      team,
		resolveProject:   project,
		resolveMilestone: milestone,
		resolveUser:      assignee,
		resolveLabels:    map[string]linear.Label{"Bug": bug, "Platform": platform},
	}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearCreate(&stdout, &stderr, bytes.NewReader(nil)),
		linearCreateFlags(),
		[]string{
			"--title", "Example issue", "--description", "Example description", "--team", "DEV",
			"--assignee", "ada@example.com", "--priority", "high", "--project", "agent-work",
			"--milestone", "Launch", "--label", "Bug", "--label", "Platform", "--dry-run",
		},
	)
	if err != nil {
		t.Fatalf("dry run returned error: %v; stderr=%s", err, stderr.String())
	}
	if client.createCalls != 0 {
		t.Errorf("dry run must not create an issue, got %d create calls", client.createCalls)
	}

	var output struct {
		DryRun    bool   `json:"dryRun"`
		Operation string `json:"operation"`
		Input     struct {
			TeamID             string   `json:"teamId"`
			AssigneeID         string   `json:"assigneeId"`
			Priority           int      `json:"priority"`
			ProjectID          string   `json:"projectId"`
			ProjectMilestoneID string   `json:"projectMilestoneId"`
			LabelIDs           []string `json:"labelIds"`
		} `json:"input"`
		Resolved linearCreateResolvedOutput `json:"resolved"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode dry-run output: %v", err)
	}
	if !output.DryRun || output.Operation != "create" || output.Input.TeamID != "team-uuid" ||
		output.Input.AssigneeID != "user-uuid" || output.Input.Priority != linearPriorityHigh ||
		output.Input.ProjectID != "project-uuid" || output.Input.ProjectMilestoneID != "milestone-uuid" ||
		!slices.Equal(output.Input.LabelIDs, []string{"label-bug", "label-platform"}) {
		t.Errorf("unexpected dry-run input: %+v", output.Input)
	}
	if output.Resolved.Team.ID != "team-uuid" || output.Resolved.Assignee == nil ||
		output.Resolved.Assignee.ID != "user-uuid" || output.Resolved.Project == nil ||
		output.Resolved.Project.ID != "project-uuid" || output.Resolved.ProjectMilestone == nil ||
		output.Resolved.ProjectMilestone.ID != "milestone-uuid" || output.Resolved.Priority == nil ||
		*output.Resolved.Priority != "high" || len(output.Resolved.Labels) != 2 {
		t.Errorf("unexpected dry-run resolutions: %+v", output.Resolved)
	}
}

func TestHandleLinearUpdatePatchesLabelsWithinIssueTeam(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	issue := newLinearIssue()
	issue.Team = team
	client := &fakeLinearClient{
		issue: issue,
		resolveLabels: map[string]linear.Label{
			"Bug":  {ID: "label-bug", Name: "Bug", Team: team},
			"Docs": {ID: "label-docs", Name: "Docs"},
		},
	}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearUpdate(&stdout, &stderr, bytes.NewReader(nil)),
		linearUpdateFlags(),
		[]string{"--add-label", "Bug", "--remove-label", "Docs", "DEV-123"},
	)
	if err != nil {
		t.Fatalf("update returned error: %v", err)
	}
	if client.getID != "DEV-123" {
		t.Errorf("expected issue read before label resolution, got %q", client.getID)
	}
	if got, want := client.updateInput.AddedLabelIDs, []string{"label-bug"}; !slices.Equal(got, want) {
		t.Errorf("expected added labels %v, got %v", want, got)
	}
	if got, want := client.updateInput.RemovedLabelIDs, []string{"label-docs"}; !slices.Equal(got, want) {
		t.Errorf("expected removed labels %v, got %v", want, got)
	}
}

func TestHandleLinearUpdateRejectsConflictingLabelFlagsBeforeMutation(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "clear and add",
			args: []string{"--clear-labels", "--add-label", "Bug", "DEV-123"},
		},
		{
			name: "clear and remove",
			args: []string{"--clear-labels", "--remove-label", "Bug", "DEV-123"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LINEAR_API_KEY", "token")
			client := &fakeLinearClient{}
			withLinearClient(t, client)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err := runLinearCommand(
				t,
				handleLinearUpdate(&stdout, &stderr, bytes.NewReader(nil)),
				linearUpdateFlags(),
				tt.args,
			)
			if err == nil {
				t.Fatal("expected a non-zero exit error")
			}
			assertLinearErrorCode(t, stderr.Bytes(), "invalid_arguments")
			if client.getID != "" || client.updateCalls != 0 {
				t.Errorf("expected no read or mutation, got get=%q updates=%d", client.getID, client.updateCalls)
			}
		})
	}
}

func TestHandleLinearUpdateRejectsTheSameResolvedLabelInBothPatches(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	team := linear.Team{ID: "team-uuid", Key: "DEV", Name: "Development"}
	issue := newLinearIssue()
	issue.Team = team
	client := &fakeLinearClient{
		issue:         issue,
		resolveLabels: map[string]linear.Label{"Bug": {ID: "label-bug", Name: "Bug", Team: team}},
	}
	withLinearClient(t, client)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runLinearCommand(
		t,
		handleLinearUpdate(&stdout, &stderr, bytes.NewReader(nil)),
		linearUpdateFlags(),
		[]string{"--add-label", "Bug", "--remove-label", "Bug", "DEV-123"},
	)
	if err == nil {
		t.Fatal("expected a non-zero exit error")
	}
	assertLinearErrorCode(t, stderr.Bytes(), "invalid_arguments")
	if client.updateCalls != 0 {
		t.Errorf("expected no mutation, got %d calls", client.updateCalls)
	}
}
