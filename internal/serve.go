package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v69/github"
	opencode "github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode-sdk-go/option"
	"github.com/thomasgormley/dev-cli-go/internal/git"
	"github.com/thomasgormley/dev-cli-go/internal/githubapi"
	"github.com/urfave/cli/v2"
)

type Comment struct {
	ID              int64  `json:"id"`
	Author          string `json:"author"`
	Body            string `json:"body"`
	CreatedAt       string `json:"createdAt"`
	ParentCommentID int64  `json:"parentCommentId,omitempty"`
}

type PRDetails struct {
	Number     int       `json:"number"`
	Title      string    `json:"title"`
	BaseBranch string    `json:"baseBranch"`
	HeadBranch string    `json:"headBranch"`
	Author     string    `json:"author"`
	State      string    `json:"state"`
	IsDraft    bool      `json:"isDraft"`
	CreatedAt  string    `json:"createdAt"`
	UpdatedAt  string    `json:"updatedAt"`
	URL        string    `json:"url"`
	Comments   []Comment `json:"comments"`
}

func parseGitHubPRURL(url string) (GitHubPR, error) {
	re := regexp.MustCompile(`github\.com/([^/]+)/([^/]+)/pulls?/(\d+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) < 4 {
		return GitHubPR{}, fmt.Errorf("invalid GitHub PR URL")
	}

	owner := matches[1]
	repo := matches[2]
	number, err := strconv.Atoi(matches[3])
	if err != nil {
		return GitHubPR{}, err
	}

	return GitHubPR{
		Owner:  owner,
		Repo:   repo,
		Number: number,
	}, nil
}

func parsePRDetails(pr *github.PullRequest, issueComments []*github.IssueComment, reviewComments []*github.PullRequestComment) PRDetails {
	var comments []Comment

	for _, ic := range issueComments {
		comments = append(comments, Comment{
			ID:        ic.GetID(),
			Author:    ic.GetUser().GetLogin(),
			Body:      ic.GetBody(),
			CreatedAt: ic.GetCreatedAt().Format(time.RFC3339),
		})
	}

	for _, rc := range reviewComments {
		parentID := int64(0)
		if replyTo := rc.GetInReplyTo(); replyTo > 0 {
			parentID = replyTo
		}
		comments = append(comments, Comment{
			ID:              rc.GetID(),
			Author:          rc.GetUser().GetLogin(),
			Body:            rc.GetBody(),
			CreatedAt:       rc.GetCreatedAt().Format(time.RFC3339),
			ParentCommentID: parentID,
		})
	}

	author := ""
	if pr.User != nil {
		author = pr.User.GetLogin()
	}

	return PRDetails{
		Number:     pr.GetNumber(),
		Title:      pr.GetTitle(),
		BaseBranch: pr.GetBase().GetRef(),
		HeadBranch: pr.GetHead().GetRef(),
		Author:     author,
		State:      pr.GetState(),
		IsDraft:    pr.GetDraft(),
		CreatedAt:  pr.GetCreatedAt().Format(time.RFC3339),
		UpdatedAt:  pr.GetUpdatedAt().Format(time.RFC3339),
		URL:        pr.GetHTMLURL(),
		Comments:   comments,
	}
}

type GitHubPR struct {
	Owner  string
	Repo   string
	Number int
}

func debugCache() PRDetails {
	return PRDetails{
		Number:     14,
		Title:      "agentic services",
		BaseBranch: "main",
		HeadBranch: "serve-cli",
		Author:     "ThomasGormley",
		State:      "open",
		IsDraft:    true,
		CreatedAt:  "2026-01-31T12:53:22Z",
		UpdatedAt:  "2026-01-31T14:39:12Z",
		URL:        "https://github.com/ThomasGormley/dev-cli-go/pull/14",
		Comments: []Comment{
			{ID: 2749631568, Author: "ThomasGormley", Body: "@ThomasGormley remove the polling logic", CreatedAt: "2026-01-31T14:39:12Z"},
		},
	}
}

func handleServe() cli.ActionFunc {

	return func(c *cli.Context) error {
		var host, port = c.String("host"), c.String("port")

		ghToken := os.Getenv("DEV_GITHUB_TOKEN")
		if ghToken == "" {
			return fmt.Errorf("DEV_GITHUB_TOKEN environment variable required")
		}
		ghClient := githubapi.NewClient(ghToken)

		srv := &http.Server{
			Addr:    net.JoinHostPort(host, port),
			Handler: handle(ghClient),
		}
		go func() {
			log.Printf("listening on %s\n", srv.Addr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, "error listening and serving: %s\n", err)
			}
		}()
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-c.Done()
			shutdownCtx := context.Background()
			shutdownCtx, cancel := context.WithTimeout(shutdownCtx, 10*time.Second)
			defer cancel()
			if err := srv.Shutdown(shutdownCtx); err != nil {
				fmt.Fprintf(os.Stderr, "error shutting down http server: %s\n", err)
			}
		}()

		wg.Wait()
		return nil
	}
}

func encode[T any](w http.ResponseWriter, status int, v T) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

func decode[T any](r *http.Request) (T, error) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		return v, fmt.Errorf("decode json: %w", err)
	}
	return v, nil
}

func handleHealth() http.Handler {
	type response struct {
		Status string `json:"status"`
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encode(w, http.StatusOK, response{Status: "ok"})
	})
}

func handlerAgentDispatch(ghClient *githubapi.Client) http.Handler {
	type request struct {
		URL string `json:"url"`
	}
	type response struct {
		Opencode string    `json:"opencode"`
		Comments []Comment `json:"comments"`
		RepoPath string    `json:"repoPath"`
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			encode(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		req, err := decode[request](r)
		if err != nil {
			encode(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		if req.URL == "" {
			encode(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
			return
		}

		prInfo, err := parseGitHubPRURL(req.URL)
		if err != nil {
			encode(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		log.Printf("PR: owner=%s repo=%s number=%d", prInfo.Owner, prInfo.Repo, prInfo.Number)

		var prDetails PRDetails
		if req.URL == "https://github.com/ThomasGormley/dev-cli-go/pull/14/" {
			log.Print("cache hit")
			prDetails = debugCache()
		} else {
			pr, issueComments, reviewComments, err := ghClient.GetPullRequestDetails(r.Context(), prInfo.Owner, prInfo.Repo, prInfo.Number)
			if err != nil {
				encode(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			prDetails = parsePRDetails(pr, issueComments, reviewComments)
		}

		var actionable []Comment
		for _, c := range prDetails.Comments {
			if c.Author == "ThomasGormley" && strings.Contains(c.Body, "@ThomasGormley") {
				actionable = append(actionable, c)
			}
		}

		repoPath := filepath.Join(os.Getenv("HOME"), ".devagent/repos", prInfo.Owner, prInfo.Repo)
		branch := prDetails.HeadBranch

		if _, err := os.Stat(repoPath); err == nil {
			git.Fetch(repoPath)
			git.CheckoutIn(branch, repoPath)
			git.ResetHardIn("origin/"+branch, repoPath)
		} else {
			os.MkdirAll(filepath.Dir(repoPath), 0755)
			cloneURL := fmt.Sprintf("git@github.com:%s/%s.git", prInfo.Owner, prInfo.Repo)
			git.Clone(cloneURL, branch, repoPath)
		}

		opencodeBaseURL := os.Getenv("OPENCODE_BASE_URL")
		if opencodeBaseURL == "" {
			opencodeBaseURL = "http://localhost:3366"
		}
		client := opencode.NewClient(option.WithBaseURL(opencodeBaseURL))
		session, err := client.Session.New(r.Context(), opencode.SessionNewParams{
			Directory: opencode.String(repoPath),
		})
		if err != nil {
			encode(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to create session: %v", err)})
			return
		}

		go func() {
			stream := client.Event.ListStreaming(context.Background(), opencode.EventListParams{})
			for stream.Next() {
				event := stream.Current()
				if event.Type == opencode.EventListResponseTypeSessionIdle {
					log.Printf("session %s idle", event.AsUnion().(opencode.EventListResponseEventSessionIdle).Properties.SessionID)
				}
			}
		}()

		for _, c := range actionable {
			promptText := strings.TrimSpace(strings.TrimPrefix(c.Body, "@"+c.Author))
			rsp, err := client.Session.Prompt(
				r.Context(),
				session.ID,
				opencode.SessionPromptParams{
					Directory: opencode.String(repoPath),
					System:    opencode.String("You are helping with a GitHub pull request. Follow instructions carefully."),
					Parts: opencode.F(
						[]opencode.SessionPromptParamsPartUnion{
							opencode.TextPartInputParam{
								Type: opencode.F(opencode.TextPartInputType("text")),
								Text: opencode.String(promptText),
							},
						}),
				},
			)
			if err != nil {
				log.Printf("failed to send prompt: %v", err)
			}
			encode(w, http.StatusOK, response{Opencode: rsp.JSON.RawJSON()})
		}

	})
}

func addRoutes(mux *http.ServeMux, ghClient *githubapi.Client) {
	mux.Handle("/api/health", handleHealth())
	mux.Handle("/api/agent/dispatch", handlerAgentDispatch(ghClient))
}

func handle(ghClient *githubapi.Client) http.Handler {
	mux := http.NewServeMux()
	addRoutes(mux, ghClient)

	var handler http.Handler = mux
	handler = cors(handler)
	handler = loggingMiddleware(handler)

	return handler
}

func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		h.ServeHTTP(w, r)
	})
}

func loggingMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s", r.Method, r.URL.Path)
		h.ServeHTTP(w, r)
	})
}
