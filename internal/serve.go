package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	opencode "github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode-sdk-go/option"
	"github.com/thomasgormley/dev-cli-go/internal/git"
	"github.com/thomasgormley/dev-cli-go/internal/githubapi"
	"github.com/urfave/cli/v2"
)

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
	return PRDetails{
		Title:      data.Repository.PullRequest.Title,
		HeadBranch: data.Repository.PullRequest.HeadRefName,
		Author:     data.Repository.PullRequest.Author.Login,
		Comments:   comments,
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

type GitHubPR struct {
	Owner  string
	Repo   string
	Number int
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

type PromptFile struct {
	Mime        string `json:"mime"`
	Content     string `json:"content"`
	Filename    string `json:"filename"`
	Replacement string `json:"replacement"`
	Start       int    `json:"start"`
	End         int    `json:"end"`
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

func handlerAgentDispatch(ghClient *githubapi.Client) http.Handler {
	type request struct {
		URL       string `json:"url"`
		CacheBust bool   `json:"cacheBust"`
	}
	type agentReply struct {
		CommentID string `json:"commentId"`
		Reply     string `json:"reply"`
	}
	type response struct {
		SessionID    string       `json:"sessionId"`
		Comments     []Comment    `json:"comments"`
		AgentReplies []agentReply `json:"agentReplies"`
		RepoPath     string       `json:"repoPath"`
		Status       string       `json:"status"`
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

		prDetails, err := fetchPRDetails(r.Context(), ghClient, prInfo, req.CacheBust)
		if err != nil {
			encode(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		if prDetails.Author != os.Getenv("DEV_AGENT_USERNAME") {
			encode(w, http.StatusForbidden, map[string]string{"error": "PR author is not authorized"})
			return
		}

		actionable := filterActionableComments(prDetails.Comments)

		branch := prDetails.HeadBranch
		repoPath, err := ensureRepo(prInfo, branch)
		if err != nil {
			encode(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		opencodeClient, sessionID, err := createOpencodeSession(r.Context(), repoPath)
		if err != nil {
			encode(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to create session: %v", err)})
			return
		}

		go func() {
			stream := opencodeClient.Event.ListStreaming(context.Background(), opencode.EventListParams{})
			for stream.Next() {
				event := stream.Current()
				if event.Type == opencode.EventListResponseTypeSessionIdle {
					log.Printf("session %s idle", event.AsUnion().(opencode.EventListResponseEventSessionIdle).Properties.SessionID)
				}
			}
		}()

		var agentReplies []agentReply
		for _, c := range actionable {
			if isProcessed(c, os.Getenv("DEV_AGENT_USERNAME")) {
				log.Printf("comment %d already processed, skipping", c.ID)
				continue
			}

			reactToComment(r.Context(), ghClient, c, prInfo)

			promptText := getPromptFromComment(c, prDetails)
			replyText, err := chat(r.Context(), opencodeClient, sessionID, promptText, repoPath)
			if err != nil {
				log.Printf("failed to send prompt: %v", err)
				continue
			}

			if commitRepoChanges(r.Context(), opencodeClient, sessionID, replyText, repoPath, branch) != nil {
				log.Printf("failed to commit repo changes")
			}

			postInlineComment(r.Context(), ghClient, c, prInfo, replyText)

			agentReplies = append(agentReplies, agentReply{
				CommentID: fmt.Sprintf("%d", c.ID),
				Reply:     replyText,
			})
		}

		status := "completed"
		if len(agentReplies) == 0 {
			status = "no_replies"
		}
		encode(w, http.StatusOK, response{SessionID: sessionID, Comments: actionable, AgentReplies: agentReplies, RepoPath: repoPath, Status: status})
	})
}

func handlerDebug(ghClient *githubapi.Client) http.Handler {
	type request struct {
		URL       string `json:"url"`
		CacheBust bool   `json:"cacheBust"`
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

		log.Printf("Debug: owner=%s repo=%s number=%d", prInfo.Owner, prInfo.Repo, prInfo.Number)

		data, err := ghClient.GetPRDetails(r.Context(), prInfo.Owner, prInfo.Repo, prInfo.Number)
		if err != nil {
			encode(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		encode(w, http.StatusOK, data)
	})
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

func filterActionableComments(comments []Comment) []Comment {
	var actionable []Comment
	for _, c := range comments {
		if c.Author == os.Getenv("DEV_AGENT_USERNAME") && strings.Contains(c.Body, "@"+os.Getenv("DEV_AGENT_USERNAME")) {
			actionable = append(actionable, c)
		}
	}
	return actionable
}

func isProcessed(c Comment, userAccount string) bool {
	for _, r := range c.Reactions {
		if strings.ToUpper(r.Content) == "EYES" && r.User == userAccount {
			return true
		}
	}
	return false
}

func ensureRepo(prInfo GitHubPR, branch string) (string, error) {
	repoPath := filepath.Join(os.Getenv("HOME"), ".devagent/repos", prInfo.Owner, prInfo.Repo)
	if _, err := os.Stat(repoPath); err == nil {
		git.Fetch(repoPath)
		git.CheckoutIn(branch, repoPath)

		cmd := exec.Command("git", "rev-parse", "--verify", "origin/"+branch)
		cmd.Dir = repoPath
		if err := cmd.Run(); err == nil {
			git.ResetHardIn("origin/"+branch, repoPath)
		} else {
			log.Printf("origin/%s not found, using local branch state", branch)
		}
		return repoPath, nil
	}
	os.MkdirAll(filepath.Dir(repoPath), 0755)
	cloneURL := fmt.Sprintf("git@github.com:%s/%s.git", prInfo.Owner, prInfo.Repo)
	git.Clone(cloneURL, branch, repoPath)
	return repoPath, nil
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

func reactToComment(ctx context.Context, ghClient *githubapi.Client, c Comment, prInfo GitHubPR) {
	isReviewComment := c.FilePath != ""
	log.Printf("adding eyes reaction to comment %d (review: %v)", c.ID, isReviewComment)
	if isReviewComment {
		if err := ghClient.CreatePullRequestCommentReaction(ctx, prInfo.Owner, prInfo.Repo, c.ID, "eyes"); err != nil {
			log.Printf("failed to add PR comment reaction: %v", err)
		} else {
			log.Printf("PR comment reaction added successfully")
		}
	} else {
		if err := ghClient.CreateReaction(ctx, prInfo.Owner, prInfo.Repo, c.ID, "eyes"); err != nil {
			log.Printf("failed to add reaction: %v", err)
		} else {
			log.Printf("reaction added successfully")
		}
	}
}

func getPromptFromComment(c Comment, prDetails PRDetails) string {
	body := strings.TrimSpace(strings.TrimPrefix(c.Body, "@"+c.Author))

	prContext := getPRContext(prDetails, c.ID)

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

	return fmt.Sprintf("%s\n\n%s", body, prContext)
}

func getPRContext(prDetails PRDetails, currentCommentID int64) string {
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

func commitRepoChanges(ctx context.Context, client *opencode.Client, sessionID string, replyText string, repoPath string, branch string) error {
	cmd := exec.Command("git", "-C", repoPath, "status", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	status := strings.TrimSpace(string(out))
	log.Printf("git status output: %s", status)
	if len(status) == 0 {
		return nil
	}

	log.Printf("branch dirty, pushing changes")
	summary, err := chat(ctx, client, sessionID, fmt.Sprintf("Summarize the following in less than 40 characters:\n\n%s", replyText), repoPath)
	if err != nil {
		summary = "auto"
	}

	if err := exec.Command("git", "-C", repoPath, "add", ".").Run(); err != nil {
		log.Printf("failed to add: %v", err)
		return err
	}
	if err := exec.Command("git", "-C", repoPath, "commit", "-m", strings.TrimSpace(summary)).Run(); err != nil {
		log.Printf("failed to commit: %v", err)
		return err
	}
	if err := exec.Command("git", "-C", repoPath, "push", "-u", "origin", branch).Run(); err != nil {
		log.Printf("failed to push: %v", err)
		return err
	}
	return nil
}

func postInlineComment(ctx context.Context, ghClient *githubapi.Client, c Comment, prInfo GitHubPR, replyText string) {
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
	} else {
		log.Printf("comment created successfully")
	}
}
func addRoutes(mux *http.ServeMux, ghClient *githubapi.Client) {
	mux.Handle("/api/health", handleHealth())
	mux.Handle("/api/debug", handlerDebug(ghClient))
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
