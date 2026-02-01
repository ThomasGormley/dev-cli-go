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
	"strings"

	"github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode-sdk-go/option"

	"github.com/thomasgormley/dev-cli-go/internal/git"
	"github.com/thomasgormley/dev-cli-go/internal/githubapi"
	"github.com/thomasgormley/dev-cli-go/internal/queuelib"
)

type HandleOpts struct {
	GitHubUser     string
	GitHubClient   *githubapi.Client
	AllowedOrigins []string
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

	q := queuelib.New[agentDispatchJob]()
	mux := http.NewServeMux()
	mux.Handle("/api/health", handleHealth(opt.OpenCode))
	mux.Handle("/api/debug", handlerDebug(opt.GitHubClient))
	mux.Handle(
		"/api/agent/dispatch",
		opencodeCheckMiddleware(opt.OpenCode,
			handlerAgentDispatch(q, opt.GitHubClient, opt.GitHubUser, opt.OpenCode),
		),
	)

	var handler http.Handler = mux
	handler = corsMiddleware(handler, opt.AllowedOrigins)
	handler = loggingMiddleware(handler)

	go func() {
		for {
			job, ok := q.Dequeue(ctx)
			if !ok {
				return
			}
			if err := handleAgentDispatchJob(ctx, opt.GitHubClient, opt.OpenCode, job); err != nil {
				log.Printf("job failed for comment %d: %v", job.comment.ID, err)
			}
		}
	}()

	return handler
}

func handleAgentDispatchJob(ctx context.Context, ghClient *githubapi.Client, config OpenCodeConfig, job agentDispatchJob) error {
	prInfo := job.prInfo
	prDetails := job.prDetails
	c := job.comment
	repoPath := job.repoPath
	sessionID := job.sessionID
	branch := job.headBranch

	if err := react(ctx, ghClient, c, prInfo); err != nil {
		return fmt.Errorf("reaction failed: %w", err)
	}

	opencodeClient := opencode.NewClient(option.WithBaseURL(fmt.Sprintf("http://%s:%s", config.Host, config.Port)))

	systemPrompt := buildPRSystemPrompt(prInfo, prDetails)

	promptText := prompt(c)
	replyText, err := promptJob(ctx, opencodeClient, sessionID, promptText, systemPrompt, repoPath, config)
	if err != nil {
		return fmt.Errorf("prompt job failed: %w", err)
	}

	commitMsg, err := promptJob(ctx, opencodeClient, sessionID, fmt.Sprintf("Summarize the following in less than 40 characters:\n\n%s", replyText), systemPrompt, repoPath, config)
	if err != nil {
		commitMsg = "auto"
	}

	repo := git.Open(repoPath)
	if err := commit(repo, branch, commitMsg); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	if err := repo.SyncToRemote(branch); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	var commentURL string
	if c.IsReviewComment() {
		commentURL = fmt.Sprintf("https://github.com/%s/%s/pull/%d#discussion_r%d", prInfo.Owner, prInfo.Repo, prInfo.Number, c.ID)
	} else {
		commentURL = fmt.Sprintf("https://github.com/%s/%s/pull/%d#issuecomment-%d", prInfo.Owner, prInfo.Repo, prInfo.Number, c.ID)
	}
	metadata := fmt.Sprintf("<details><summary>👉 info</summary>\n\n| Key | Value |\n|-----|-------|\n| In reply to | [#%d](%s) |\n| Session | `%s` |\n\n</details>", c.ID, commentURL, sessionID)
	if err := comment(ctx, ghClient, c, prInfo, replyText+"\n\n"+metadata); err != nil {
		return fmt.Errorf("comment failed: %w", err)
	}

	log.Printf("comment %d processed successfully", c.ID)
	return nil
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

func fetchPRDetails(ctx context.Context, ghClient *githubapi.Client, prInfo GitHubPR) (PRDetails, error) {
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

func filterActionableComments(comments []Comment, user string) []Comment {
	var actionable []Comment
	for _, c := range comments {
		if c.Author != user {
			continue
		}
		if !strings.Contains(c.Body, "@"+user) {
			continue
		}

		if hasEyesReaction(c.Reactions, user) {
			continue
		}

		actionable = append(actionable, c)
	}
	return actionable
}

func hasEyesReaction(reactions []Reaction, user string) bool {
	for _, r := range reactions {
		if strings.ToUpper(r.Content) == "EYES" && r.User == user {
			return true
		}
	}
	return false
}

func react(ctx context.Context, ghClient *githubapi.Client, c Comment, prInfo GitHubPR) error {
	isReviewComment := c.IsReviewComment()
	log.Printf("adding eyes reaction to comment %d (review: %v)", c.ID, isReviewComment)
	if isReviewComment {
		if err := ghClient.CreatePullRequestCommentReaction(ctx, prInfo.Owner, prInfo.Repo, c.ID, "eyes"); err != nil {
			log.Printf("failed to add PR comment reaction: %v", err)
			return err
		}
	} else {
		if err := ghClient.CreateReaction(ctx, prInfo.Owner, prInfo.Repo, c.ID, "eyes"); err != nil {
			log.Printf("failed to add reaction: %v", err)
			return err
		}
	}
	log.Printf("reaction added successfully")
	return nil
}

func prompt(c Comment) string {
	body := strings.TrimSpace(strings.TrimPrefix(c.Body, "@"+c.Author))

	if !c.IsReviewComment() {
		return body
	}

	commentContext := fmt.Sprintf("You are reviewing a comment on file \"%s\" at line %d.", c.FilePath, c.Line)
	if c.DiffHunk != "" {
		commentContext = fmt.Sprintf("You are reviewing a comment on file \"%s\" at line %d.\n\nDiff context:\n%s", c.FilePath, c.Line, c.DiffHunk)
	}

	return fmt.Sprintf("%s\n\n%s", commentContext, body)
}

func commit(repo git.Repo, branch string, commitMessage string) error {
	status, err := repo.Status()
	if err != nil {
		return err
	}

	log.Printf("git status output: %s", status)
	if len(status) == 0 {
		return nil
	}

	log.Printf("branch dirty, pushing changes")

	if err := repo.Add("."); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	if err := repo.Commit(commitMessage); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	if err := repo.Push("origin", branch); err != nil {
		return fmt.Errorf("git push: %w", err)
	}
	return nil
}

func comment(ctx context.Context, ghClient *githubapi.Client, c Comment, prInfo GitHubPR, replyText string) error {
	log.Printf("creating comment to PR %d", prInfo.Number)
	var err error
	isReviewComment := c.IsReviewComment()
	if isReviewComment {
		log.Printf("creating review reply to comment %d", c.ID)
		err = ghClient.CreateReviewCommentReply(ctx, prInfo.Owner, prInfo.Repo, prInfo.Number, c.ID, replyText)
	} else {
		err = ghClient.CreateComment(ctx, prInfo.Owner, prInfo.Repo, prInfo.Number, replyText)
	}
	if err != nil {
		log.Printf("failed to create comment: %v", err)
		return err
	}
	log.Printf("comment created successfully")
	return nil
}
