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
	DiffHunk        string `json:"diffHunk,omitempty"`
	FilePath        string `json:"filePath,omitempty"`
	Line            int    `json:"line,omitempty"`
	StartLine       int    `json:"startLine,omitempty"`
	OriginalLine    int    `json:"originalLine,omitempty"`
	OriginalPos     int    `json:"originalPos,omitempty"`
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
			DiffHunk:        rc.GetDiffHunk(),
			FilePath:        rc.GetPath(),
			Line:            int(rc.GetLine()),
			StartLine:       int(rc.GetStartLine()),
			OriginalLine:    int(rc.GetOriginalLine()),
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

func extractTextFromResponse(rsp *opencode.SessionPromptResponse) string {
	var textParts []string
	for _, part := range rsp.Parts {
		if part.Type == opencode.PartTypeText && part.Text != "" {
			textParts = append(textParts, part.Text)
		}
	}
	return strings.Join(textParts, "\n")
}

func chat(ctx context.Context, client *opencode.Client, sessionID string, text string, directory string, files []PromptFile) (string, error) {
	log.Printf("Sending message to opencode...")

	var parts []opencode.SessionPromptParamsPartUnion
	parts = append(parts, opencode.TextPartInputParam{
		Type: opencode.F(opencode.TextPartInputType("text")),
		Text: opencode.String(text),
	})

	for _, f := range files {
		textParam := opencode.FilePartSourceParam{
			Type: opencode.F(opencode.FilePartSourceType("file")),
			Text: opencode.F(opencode.FilePartSourceTextParam{
				Start: opencode.Int(int64(f.Start)),
				End:   opencode.Int(int64(f.End)),
				Value: opencode.String(f.Replacement),
			}),
			Path: opencode.String(f.Filename),
		}
		parts = append(parts, opencode.FilePartInputParam{
			Type:     opencode.F(opencode.FilePartInputType("file")),
			Mime:     opencode.String(f.Mime),
			URL:      opencode.String(fmt.Sprintf("data:%s;base64,%s", f.Mime, f.Content)),
			Filename: opencode.String(f.Filename),
			Source:   opencode.Raw[opencode.FilePartSourceUnionParam](textParam),
		})
	}

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

	replyText := extractTextFromResponse(rsp)
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

		cachePath := filepath.Join(".cache", fmt.Sprintf("pr%d.json", prInfo.Number))
		var prDetails PRDetails

		if req.CacheBust {
			log.Print("cache bust: fetching real data")
			pr, issueComments, reviewComments, err := ghClient.GetPullRequestDetails(r.Context(), prInfo.Owner, prInfo.Repo, prInfo.Number)
			if err != nil {
				encode(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			prDetails = parsePRDetails(pr, issueComments, reviewComments)
			os.MkdirAll(filepath.Dir(cachePath), 0755)
			cacheData, _ := json.MarshalIndent(prDetails, "", "  ")
			os.WriteFile(cachePath, cacheData, 0644)
			log.Printf("cache updated: %s", cachePath)
		} else {
			if data, err := os.ReadFile(cachePath); err == nil {
				log.Printf("cache hit: %s", cachePath)
				json.Unmarshal(data, &prDetails)
			} else {
				log.Print("cache miss: fetching real data")
				pr, issueComments, reviewComments, err := ghClient.GetPullRequestDetails(r.Context(), prInfo.Owner, prInfo.Repo, prInfo.Number)
				if err != nil {
					encode(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}
				prDetails = parsePRDetails(pr, issueComments, reviewComments)
			}
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

		var agentReplies []agentReply
		for _, c := range actionable {
			promptText := strings.TrimSpace(strings.TrimPrefix(c.Body, "@"+c.Author))
			replyText, err := chat(r.Context(), client, session.ID, promptText, repoPath, []PromptFile{})
			if err != nil {
				log.Printf("failed to send prompt: %v", err)
				continue
			}
			agentReplies = append(agentReplies, agentReply{
				CommentID: fmt.Sprintf("%d", c.ID),
				Reply:     replyText,
			})

		}

		status := "completed"
		if len(agentReplies) == 0 {
			status = "no_replies"
		}
		encode(w, http.StatusOK, response{SessionID: session.ID, Comments: actionable, AgentReplies: agentReplies, RepoPath: repoPath, Status: status})
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
