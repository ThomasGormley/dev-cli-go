package serve

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/sst/opencode-sdk-go"

	"github.com/thomasgormley/dev-cli-go/internal/git"
	"github.com/thomasgormley/dev-cli-go/internal/githubapi"
	"github.com/thomasgormley/dev-cli-go/internal/queuelib"
)

func handlerAgentDispatch(queue queuelib.Queue[agentDispatchJob], ghClient githubapi.Client, opencodeClient opencode.Client, user string, opencodeConfig OpenCodeConfig) http.Handler {
	type request struct {
		URL string `json:"url"`
	}
	type response struct {
		SessionID string    `json:"sessionId"`
		Comments  []Comment `json:"comments"`
		RepoPath  string    `json:"repoPath"`
		Status    string    `json:"status"`
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

		prDetails, err := fetchPRDetails(r.Context(), ghClient, prInfo)
		if err != nil {
			encode(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		if prDetails.Author != user {
			encode(w, http.StatusForbidden, map[string]string{"error": "PR author is not authorized"})
			return
		}

		actionable := filterActionableComments(prDetails.Comments, user)

		branch := prDetails.HeadBranch
		repo, err := git.EnsureClone(r.Context(), prInfo.Owner, prInfo.Repo, branch)
		if err != nil {
			encode(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		sessionID, err := createOpencodeSession(r.Context(), opencodeClient, repo.Path(), opencodeConfig)
		if err != nil {
			encode(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to create session: %v", err)})
			return
		}

		enqueued := 0
		for _, c := range actionable {
			job := agentDispatchJob{
				prInfo:     prInfo,
				prDetails:  prDetails,
				headBranch: prDetails.HeadBranch,
				comment:    c,
				repoPath:   repo.Path(),
				sessionID:  sessionID,
				user:       user,
			}
			if err := queue.Enqueue(r.Context(), job); err != nil {
				log.Printf("failed to enqueue comment %d: %v", c.ID, err)
				continue
			}
			enqueued++
		}

		status := "accepted"
		if enqueued == 0 {
			status = "no_actionable"
		}
		encode(w, http.StatusAccepted, response{SessionID: sessionID, Comments: actionable, RepoPath: repo.Path(), Status: status})
	})
}

func processAgentDispatchQueue(ctx context.Context, queue queuelib.Queue[agentDispatchJob], ghClient githubapi.Client, opencodeClient opencode.Client, config OpenCodeConfig) {
	for {
		job, ok := queue.Dequeue(ctx)
		if !ok {
			return
		}
		if err := runAgentDispatchJob(ctx, ghClient, opencodeClient, config, job); err != nil {
			log.Printf("job failed for comment %d: %v", job.comment.ID, err)
		}
	}
}

func runAgentDispatchJob(ctx context.Context, ghClient githubapi.Client, opencodeClient opencode.Client, config OpenCodeConfig, job agentDispatchJob) error {
	var (
		prInfo    = job.prInfo
		prDetails = job.prDetails
		repoPath  = job.repoPath
		sessionID = job.sessionID
		branch    = job.headBranch
		c         = job.comment
	)

	if err := react(ctx, ghClient, c, prInfo); err != nil {
		return fmt.Errorf("reacting: %w", err)
	}

	systemPrompt := buildPRSystemPrompt(prDetails)

	replyText, err := chat(ctx, opencodeClient, sessionID, prompt(c), systemPrompt, repoPath, config)
	if err != nil {
		return fmt.Errorf("prompting job: %w", err)
	}

	commitMsg, err := chat(ctx,
		opencodeClient,
		sessionID,
		fmt.Sprintf("Summarize the following in less than 40 characters for the purposes of a commit message:\n\n%s", replyText),
		systemPrompt,
		repoPath,
		config,
	)
	if err != nil {
		commitMsg = "auto"
	}

	repo := git.Open(repoPath)
	if err := commit(repo, branch, commitMsg); err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	if err := repo.SyncToRemote(branch); err != nil {
		return fmt.Errorf("syncing: %w", err)
	}

	var commentURL string
	if c.IsReviewComment() {
		commentURL = fmt.Sprintf("https://github.com/%s/%s/pull/%d#discussion_r%d", prInfo.Owner, prInfo.Repo, prInfo.Number, c.ID)
	} else {
		commentURL = fmt.Sprintf("https://github.com/%s/%s/pull/%d#issuecomment-%d", prInfo.Owner, prInfo.Repo, prInfo.Number, c.ID)
	}
	metadata := fmt.Sprintf("<details><summary>info</summary>\n\n| Key | Value |\n|-----|-------|\n| In reply to | [#%d](%s) |\n| Session | `%s` |\n\n</details>", c.ID, commentURL, sessionID)
	if err := comment(ctx, ghClient, c, prInfo, replyText+"\n\n"+metadata); err != nil {
		return fmt.Errorf("commenting: %w", err)
	}

	log.Printf("comment %d processed successfully", c.ID)
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
		return fmt.Errorf("adding: %w", err)
	}
	if err := repo.Commit(commitMessage); err != nil {
		return fmt.Errorf("committing changes: %w", err)
	}
	if err := repo.Push("origin", branch); err != nil {
		return fmt.Errorf("pushing: %w", err)
	}
	return nil
}

func comment(ctx context.Context, ghClient githubapi.Client, c Comment, prInfo GitHubPR, replyText string) error {
	log.Printf("creating comment to PR %d", prInfo.Number)
	var err error
	if c.IsReviewComment() {
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

func react(ctx context.Context, ghClient githubapi.Client, c Comment, prInfo GitHubPR) error {
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

func chat(ctx context.Context, client opencode.Client, sessionID string, taskText string, systemPrompt string, repoPath string, config OpenCodeConfig) (string, error) {
	var parts []opencode.SessionPromptParamsPartUnion
	parts = append(parts, opencode.TextPartInputParam{
		Type: opencode.F(opencode.TextPartInputType("text")),
		Text: opencode.String(taskText),
	})

	rsp, err := client.Session.Prompt(
		ctx,
		sessionID,
		opencode.SessionPromptParams{
			Directory: opencode.String(repoPath),
			Model: opencode.F(opencode.SessionPromptParamsModel{
				ProviderID: opencode.String(config.Provider),
				ModelID:    opencode.String(config.Model),
			}),
			System: opencode.String(systemPrompt),
			Parts:  opencode.F(parts),
		},
	)
	if err != nil {
		return "", fmt.Errorf("prompting job: %w", err)
	}

	var textParts []string
	for _, part := range rsp.Parts {
		if part.Type == opencode.PartTypeText && part.Text != "" {
			textParts = append(textParts, part.Text)
		}
	}
	replyText := strings.Join(textParts, "\n")
	if replyText == "" {
		return "", fmt.Errorf("parsing response")
	}

	return replyText, nil
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

func buildPRSystemPrompt(prDetails PRDetails) string {
	return fmt.Sprintf(`Instructions:
- You are helping with a GitHub pull request
- Read each comment and execute the requested task
- Commit changes with clear, concise messages
- Reply to comments with your changes and include session metadata
- Use bash to make changes, then commit and push

PR Details:
- PR #%d: %s
- Author: %s
- Base: %s <- Head: %s
%s- Changes: +%d/-%d

Changed Files:
%s

Previous Comments:
%s`,
		prDetails.Number, prDetails.Title,
		prDetails.Author,
		prDetails.BaseBranch, prDetails.HeadBranch,
		descriptionLine(prDetails.Body),
		prDetails.Additions, prDetails.Deletions,
		fileList(prDetails.Files),
		commentList(prDetails.Comments),
	)
}

func descriptionLine(body string) string {
	if body == "" {
		return ""
	}
	return fmt.Sprintf("- Description: %s\n", body)
}

func fileList(files []PRFile) string {
	if len(files) == 0 {
		return ""
	}
	var lines []string
	for _, f := range files {
		lines = append(lines, fmt.Sprintf("- %s (%s) +%d/-%d", f.Path, f.ChangeType, f.Additions, f.Deletions))
	}
	return strings.Join(lines, "\n") + "\n"
}

func commentList(comments []Comment) string {
	var lines []string
	for _, c := range comments {
		var location string
		if c.IsReviewComment() {
			location = fmt.Sprintf(" (%s:%d)", c.FilePath, c.Line)
		}
		lines = append(lines, fmt.Sprintf("- %s at %s%s: %s", c.Author, c.CreatedAt, location, c.Body))
	}
	return strings.Join(lines, "\n")
}
