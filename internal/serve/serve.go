package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	opencode "github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode-sdk-go/option"
	"github.com/thomasgormley/dev-cli-go/internal/git"
	"github.com/thomasgormley/dev-cli-go/internal/githubapi"
)

type HandleOpts struct {
	GitHubUser     string
	GitHubClient   *githubapi.Client
	AllowedOrigins []string
}

func Handle(opt HandleOpts) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/health", handleHealth())
	mux.Handle("/api/debug", handlerDebug(opt.GitHubClient))
	mux.Handle("/api/agent/dispatch", handlerAgentDispatch(opt.GitHubClient, opt.GitHubUser))

	var handler http.Handler = mux
	handler = corsMiddleware(handler, opt.AllowedOrigins)
	handler = loggingMiddleware(handler)

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

func loggingMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s", r.Method, r.URL.Path)
		h.ServeHTTP(w, r)
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

func parseGraphQLPR(data *githubapi.GraphQLPRResponse) PRDetails {
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
	}
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

func fetchPRDetails(ctx context.Context, ghClient *githubapi.Client, prInfo GitHubPR, cacheBust bool) (PRDetails, error) {
	cachePath := filepath.Join(".cache", fmt.Sprintf("pr%d.json", prInfo.Number))

	if cacheBust {
		log.Print("cache bust: fetching real data")
		data, err := ghClient.GetPRDetails(ctx, prInfo.Owner, prInfo.Repo, prInfo.Number)
		if err != nil {
			return PRDetails{}, err
		}
		prDetails := parseGraphQLPR(data)
		os.MkdirAll(filepath.Dir(cachePath), 0755)
		cacheData, _ := json.MarshalIndent(prDetails, "", "  ")
		os.WriteFile(cachePath, cacheData, 0644)
		log.Printf("cache updated: %s", cachePath)
		return prDetails, nil
	}

	if os.Getenv("USE_CACHE") == "true" {
		if data, err := os.ReadFile(cachePath); err == nil {
			log.Printf("cache hit: %s", cachePath)
			var prDetails PRDetails
			json.Unmarshal(data, &prDetails)
			return prDetails, nil
		}
	}

	log.Print("cache miss: fetching real data")
	data, err := ghClient.GetPRDetails(ctx, prInfo.Owner, prInfo.Repo, prInfo.Number)
	if err != nil {
		return PRDetails{}, err
	}
	return parseGraphQLPR(data), nil
}

func filterActionableComments(comments []Comment, user string) []Comment {
	var actionable []Comment
	for _, c := range comments {
		if c.Author == user && strings.Contains(c.Body, "@"+user) {
			actionable = append(actionable, c)
		}
	}
	return actionable
}

func isProcessed(c Comment, userAccount string, processedThisRun map[int64]bool) bool {
	if processedThisRun[c.ID] {
		return true
	}
	for _, r := range c.Reactions {
		if strings.ToUpper(r.Content) == "EYES" && r.User == userAccount {
			return true
		}
	}
	return false
}

func createOpencodeSession(ctx context.Context, repoPath string) (*opencode.Client, string, error) {
	opencodeBaseURL := os.Getenv("OPENCODE_BASE_URL")
	if opencodeBaseURL == "" {
		opencodeBaseURL = "http://localhost:3366"
	}
	client := opencode.NewClient(option.WithBaseURL(opencodeBaseURL))
	session, err := client.Session.New(ctx, opencode.SessionNewParams{
		Directory: opencode.String(repoPath),
	})
	if err != nil {
		return nil, "", err
	}
	return client, session.ID, nil
}

func react(ctx context.Context, ghClient *githubapi.Client, c Comment, prInfo GitHubPR) error {
	isReviewComment := c.FilePath != ""
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

func prompt(c Comment, prDetails PRDetails) string {
	body := strings.TrimSpace(strings.TrimPrefix(c.Body, "@"+c.Author))

	prContext := prContext(prDetails, c.ID)
	prFiles := prFiles(prDetails)

	isReviewComment := c.FilePath != ""

	if isReviewComment {
		var commentContext string
		if c.DiffHunk != "" {
			commentContext = fmt.Sprintf("You are reviewing a comment on file \"%s\" at line %d.\n\nDiff context:\n%s", c.FilePath, c.Line, c.DiffHunk)
		} else {
			commentContext = fmt.Sprintf("You are reviewing a comment on file \"%s\" at line %d.", c.FilePath, c.Line)
		}

		return fmt.Sprintf("%s\n\n%s\n\n%s", body, prContext, commentContext)
	}

	return fmt.Sprintf("%s\n\n%s\n\n%s", body, prContext, prFiles)
}

func prFiles(prDetails PRDetails) string {
	if len(prDetails.Files) == 0 {
		return ""
	}
	var parts []string
	parts = append(parts, "== Changed Files ==")
	for _, f := range prDetails.Files {
		parts = append(parts, fmt.Sprintf("- %s (%s) +%d/-%d", f.Path, f.ChangeType, f.Additions, f.Deletions))
	}
	return strings.Join(parts, "\n")
}

func prContext(prDetails PRDetails, currentCommentID int64) string {
	var parts []string

	parts = append(parts, "== Previous discussion context ==")

	var prevComments []string
	for _, cm := range prDetails.Comments {
		if cm.ID == currentCommentID {
			continue
		}
		var location string
		if cm.FilePath != "" {
			location = fmt.Sprintf(" (%s:%d)", cm.FilePath, cm.Line)
		}
		prevComments = append(prevComments, fmt.Sprintf("- %s at %s%s: %s", cm.Author, cm.CreatedAt, location, cm.Body))
	}

	if len(prevComments) > 0 {
		parts = append(parts, "<previous_comments>")
		parts = append(parts, prevComments...)
		parts = append(parts, "</previous_comments>")
	}

	return strings.Join(parts, "\n")
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
	isReviewComment := c.FilePath != ""
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

func textFromRsp(rsp *opencode.SessionPromptResponse) string {
	var textParts []string
	for _, part := range rsp.Parts {
		if part.Type == opencode.PartTypeText && part.Text != "" {
			textParts = append(textParts, part.Text)
		}
	}
	return strings.Join(textParts, "\n")
}

func chat(ctx context.Context, client *opencode.Client, sessionID string, text string, directory string) (string, error) {
	log.Printf("Sending message to opencode...")

	var parts []opencode.SessionPromptParamsPartUnion
	parts = append(parts, opencode.TextPartInputParam{
		Type: opencode.F(opencode.TextPartInputType("text")),
		Text: opencode.String(text),
	})

	rsp, err := client.Session.Prompt(
		ctx,
		sessionID,
		opencode.SessionPromptParams{
			Directory: opencode.String(directory),
			Model: opencode.F(opencode.SessionPromptParamsModel{
				ProviderID: opencode.String("opencode"),
				ModelID:    opencode.String("minimax-m2.1-free"),
			}),
			System: opencode.String("You are helping with a GitHub pull request. Follow instructions carefully."),
			Parts:  opencode.F(parts),
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to send prompt: %w", err)
	}

	replyText := textFromRsp(rsp)
	if replyText == "" {
		return "", fmt.Errorf("failed to parse the text response")
	}

	return replyText, nil
}
