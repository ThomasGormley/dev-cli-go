package gh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type GitHubClienter interface {
	AuthStatus() error
	CreatePR(title, body, base string, draft bool) error
	ViewPR(identifier string) error
	PRStatus(identifier string) (PRStatusResponse, error)
	MergePR(s MergeStrategy) error
}

type ghClient struct {
	Stderr io.Writer
	Stdout io.Writer
	Stdin  io.Reader
}

func (g *ghClient) AuthStatus() error {
	cmd := g.prepareCmd("gh", "auth", "status")
	cmd.Stdout = nil // gh auth status writes to stdout, we don't need to see it
	err := cmd.Run()
	if err != nil {
		return err
	}
	return err
}

func (g *ghClient) CreatePR(title, body, base string, draft bool) error {
	args := []string{"pr", "create", "--title", title, "--body", body, "--base", base}
	if draft {
		args = append(args, "--draft")
	}
	cmd := g.prepareCmd("gh", args...)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func (g *ghClient) ViewPR(identifier string) error {
	args := []string{"pr", "view"}
	if identifier != "" {
		args = append(args, identifier)
	}
	cmd := g.prepareCmd("gh", append(args, "--web")...)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

// potential fields
// currentBranch
// 		baseRefName
// 		comments
// 		headRefName
// 		files (additions/deletions check?)
// 		isDraft
// 		latestReviews
// 		mergeStateStatus
// 		mergeable
// 		mergeable
// 		statusCheckRollup
// 		statusCheckRollup

type MergeStateStatus string

const (

	// The head ref is out of date.
	BEHIND MergeStateStatus = "BEHIND"

	// The merge is blocked.
	BLOCKED MergeStateStatus = "BLOCKED"

	// Mergeable and passing commit status.
	CLEAN MergeStateStatus = "CLEAN"

	// The merge commit cannot be cleanly created.
	DIRTY MergeStateStatus = "DIRTY"

	// The merge is blocked due to the pull request being a draft.
	DRAFT MergeStateStatus = "DRAFT"

	// Mergeable with passing commit status and pre-receive hooks.
	HAS_HOOKS MergeStateStatus = "HAS_HOOKS"

	// The state cannot currently be determined.
	UNKNOWN MergeStateStatus = "UNKNOWN"

	// Mergeable with non-passing commit status.
	UNSTABLE MergeStateStatus = "UNSTABLE"
)

type MergeStrategy string

const (
	// The head ref is out of date.
	MergeSquash MergeStrategy = "squash"
	// The merge is blocked.
	MergeCommit MergeStrategy = "merge"
	// Mergeable and passing commit status.
	MergeRebase MergeStrategy = "rebase"
)

type PRStatusResponse struct {
	CurrentBranch struct {
		Closed       bool   `json:"closed"`
		Additions    int    `json:"additions"`
		BaseRefName  string `json:"baseRefName"`
		ChangedFiles int    `json:"changedFiles"`
		HeadRefName  string `json:"headRefName"`
		IsDraft      bool   `json:"isDraft"`
		Comments     []struct {
			ID     string `json:"id"`
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
			Body         string `json:"body"`
			CreatedAt    string `json:"createdAt"`
			IncludesEdit bool   `json:"includesCreatedEdit"`
			URL          string `json:"url"`
		} `json:"comments"`
		Commits []struct {
			AuthoredDate string `json:"authoredDate"`
			OID          string `json:"oid"`
		} `json:"commits"`
		Files []struct {
			Path      string `json:"path"`
			Additions int    `json:"additions"`
			Deletions int    `json:"deletions"`
		} `json:"files"`
		MergeStateStatus  MergeStateStatus    `json:"mergeStateStatus"`
		Mergeable         string              `json:"mergeable"`
		StatusCheckRollup []StatusCheckRollup `json:"statusCheckRollup"`
		Title             string              `json:"title"`
		UpdatedAt         string              `json:"updatedAt"`
		URL               string              `json:"url"`
	} `json:"currentBranch"`
}

type StatusCheckRollup struct {
	Name       string `json:"name"`
	Context    string `json:"context"`
	Conclusion string `json:"conclusion"`
	DetailsURL string `json:"detailsUrl"`
	Status     string `json:"status"`
	State      string `json:"state"`
}

var jsonFields = []string{
	"additions", "baseRefName", "changedFiles", "headRefName", "isDraft",
	"comments", "commits", "files", "mergeStateStatus", "mergeable",
	"statusCheckRollup", "title", "updatedAt", "url", "closed",
}

func (g *ghClient) PRStatus(identifier string) (PRStatusResponse, error) {
	args := []string{"pr", "status"}
	if identifier != "" {
		args = append(args, identifier)
	}
	jsonFieldArg := fmt.Sprintf("--json=%s", strings.Join(jsonFields, ","))
	var outBuffer bytes.Buffer
	cmd := g.prepareCmd("gh", append(args, jsonFieldArg)...)
	cmd.Stdout = &outBuffer
	err := cmd.Run()
	if err != nil {
		return PRStatusResponse{}, err
	}

	var resp PRStatusResponse
	if err := json.Unmarshal(outBuffer.Bytes(), &resp); err != nil {
		return PRStatusResponse{}, err
	}

	if len(resp.CurrentBranch.Commits) == 0 {
		return PRStatusResponse{}, fmt.Errorf("no pull request available")
	}
	return resp, nil
}

func (g *ghClient) MergePR(strategy MergeStrategy) error {
	args := []string{"pr", "merge"}
	switch strategy {
	case MergeSquash:
		args = append(args, "--squash")
	case MergeCommit:
		args = append(args, "--merge")
	case MergeRebase:
		args = append(args, "--rebase")
	default:
		return fmt.Errorf("missing or invalid merge strategy %s", strategy)
	}
	cmd := g.prepareCmd("gh", args...)
	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	cmd.Stdin = nil

	// cmd.Env = append(cmd.Env, "GH_PROMPT_DISABLED=true")
	return cmd.Run()
}

func (g *ghClient) prepareCmd(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Stdout = g.Stdout
	cmd.Stdin = g.Stdin
	cmd.Stderr = g.Stderr

	return cmd
}

var CmdCtx = func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

func NewGitHubClient(stderr, stdout io.Writer, stdin io.Reader) *ghClient {
	return &ghClient{
		Stderr: stderr,
		Stdout: stdout,
		Stdin:  stdin,
	}
}
