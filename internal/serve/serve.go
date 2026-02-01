package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strconv"

	"github.com/sst/opencode-sdk-go"
	"github.com/thomasgormley/dev-cli-go/internal/githubapi"
	"github.com/thomasgormley/dev-cli-go/internal/queuelib"
)

type HandleOpts struct {
	GitHubUser     string
	GitHubClient   githubapi.Client
	AllowedOrigins []string
	OpenCodeClient opencode.Client
	OpenCode       OpenCodeConfig
}

type OpenCodeConfig struct {
	Provider string
	Model    string
	Host     string
	Port     string
}

type agentDispatchJob struct {
	prInfo     GitHubPR
	prDetails  PRDetails
	headBranch string
	comment    Comment
	repoPath   string
	sessionID  string
	user       string
}

func Handle(ctx context.Context, opt HandleOpts) http.Handler {

	queue := queuelib.New[agentDispatchJob]()
	mux := http.NewServeMux()
	mux.Handle("/api/health", handleHealth(opt.OpenCode))
	mux.Handle("/api/debug", handlerDebug(opt.GitHubClient))
	mux.Handle(
		"/api/agent/dispatch",
		opencodeCheckMiddleware(opt.OpenCode,
			handlerAgentDispatch(
				queue,
				opt.GitHubClient,
				opt.OpenCodeClient,
				opt.GitHubUser,
				opt.OpenCode,
			),
		),
	)

	var handler http.Handler = mux
	handler = corsMiddleware(handler, opt.AllowedOrigins)
	handler = loggingMiddleware(handler)

	go processAgentDispatchQueue(ctx, queue, opt.GitHubClient, opt.OpenCodeClient, opt.OpenCode)

	return handler
}

func corsMiddleware(h http.Handler, allowedOrigins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if slices.Contains(allowedOrigins, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		h.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func loggingMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s", r.Method, r.URL.Path)
		rec := &statusRecorder{ResponseWriter: w}
		h.ServeHTTP(rec, r)
		if rec.status >= 400 {
			log.Printf("[%d] %s %s", rec.status, r.Method, r.URL.Path)
		}
	})
}

func opencodeCheckMiddleware(config OpenCodeConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsOpenCodeRunning(config.Host, config.Port) {
			encode(w, http.StatusServiceUnavailable, map[string]string{"error": "OpenCode not available"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

type Reaction struct {
	Content string `json:"content"`
	User    string `json:"user"`
}

type Comment struct {
	ID              int64      `json:"id"`
	Author          string     `json:"author"`
	Body            string     `json:"body"`
	CreatedAt       string     `json:"createdAt"`
	ParentCommentID int64      `json:"parentCommentId,omitempty"`
	DiffHunk        string     `json:"diffHunk,omitempty"`
	FilePath        string     `json:"filePath,omitempty"`
	Line            int        `json:"line,omitempty"`
	StartLine       int        `json:"startLine,omitempty"`
	OriginalLine    int        `json:"originalLine,omitempty"`
	OriginalPos     int        `json:"originalPos,omitempty"`
	Reactions       []Reaction `json:"reactions,omitempty"`
}

func (c Comment) IsReviewComment() bool {
	return c.FilePath != ""
}

type PRDetails struct {
	Number     int       `json:"number"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	BaseBranch string    `json:"baseBranch"`
	HeadBranch string    `json:"headBranch"`
	Author     string    `json:"author"`
	State      string    `json:"state"`
	IsDraft    bool      `json:"isDraft"`
	Additions  int       `json:"additions"`
	Deletions  int       `json:"deletions"`
	Commits    int       `json:"commits"`
	CreatedAt  string    `json:"createdAt"`
	UpdatedAt  string    `json:"updatedAt"`
	URL        string    `json:"url"`
	Comments   []Comment `json:"comments"`
	Files      []PRFile  `json:"files"`
}

type PRFile struct {
	Path       string `json:"path"`
	ChangeType string `json:"changeType"`
	Additions  int    `json:"additions"`
	Deletions  int    `json:"deletions"`
}

type GitHubPR struct {
	Owner  string
	Repo   string
	Number int
}

type PromptFile struct {
	Mime        string `json:"mime"`
	Content     string `json:"content"`
	Filename    string `json:"filename"`
	Replacement string `json:"replacement"`
	Start       int    `json:"start"`
	End         int    `json:"end"`
}

func encode[T any](w http.ResponseWriter, status int, v T) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return fmt.Errorf("encoding json: %w", err)
	}
	return nil
}

func decode[T any](r *http.Request) (T, error) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		return v, fmt.Errorf("decoding json: %w", err)
	}
	return v, nil
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

func convertReactions(reactions []struct {
	Content string `json:"content"`
	User    struct {
		Login string `json:"login"`
	}
}) []Reaction {
	var r []Reaction
	for _, rc := range reactions {
		r = append(r, Reaction{
			Content: rc.Content,
			User:    rc.User.Login,
		})
	}
	return r
}

func fetchPRDetails(ctx context.Context, ghClient githubapi.Client, prInfo GitHubPR) (PRDetails, error) {
	data, err := ghClient.GetPRDetails(ctx, prInfo.Owner, prInfo.Repo, prInfo.Number)
	if err != nil {
		return PRDetails{}, err
	}

	var comments []Comment
	for _, c := range data.Repository.PullRequest.Comments.Nodes {
		comments = append(comments, Comment{
			ID:        c.DatabaseID,
			Author:    c.Author.Login,
			Body:      c.Body,
			CreatedAt: c.CreatedAt,
			Reactions: convertReactions(c.Reactions.Nodes),
		})
	}
	for _, review := range data.Repository.PullRequest.Reviews.Nodes {
		for _, c := range review.Comments.Nodes {
			comments = append(comments, Comment{
				ID:           c.DatabaseID,
				Author:       c.Author.Login,
				Body:         c.Body,
				CreatedAt:    c.CreatedAt,
				FilePath:     c.Path,
				Line:         c.Line,
				StartLine:    c.StartLine,
				OriginalLine: c.OriginalLine,
				OriginalPos:  c.OriginalPosition,
				DiffHunk:     c.DiffHunk,
				Reactions:    convertReactions(c.Reactions.Nodes),
			})
		}
	}
	var files []PRFile
	for _, f := range data.Repository.PullRequest.Files.Nodes {
		files = append(files, PRFile{
			Path:       f.Path,
			ChangeType: f.ChangeType,
			Additions:  f.Additions,
			Deletions:  f.Deletions,
		})
	}

	return PRDetails{
		Title:      data.Repository.PullRequest.Title,
		Body:       data.Repository.PullRequest.Body,
		HeadBranch: data.Repository.PullRequest.HeadRefName,
		BaseBranch: data.Repository.PullRequest.BaseRefName,
		Author:     data.Repository.PullRequest.Author.Login,
		Additions:  data.Repository.PullRequest.Additions,
		Deletions:  data.Repository.PullRequest.Deletions,
		Commits:    data.Repository.PullRequest.Commits.TotalCount,
		Comments:   comments,
		Files:      files,
	}, nil
}
