package serve

import (
	"context"
	"fmt"
	"log"
	"net/http"

	opencode "github.com/sst/opencode-sdk-go"
	"github.com/thomasgormley/dev-cli-go/internal/git"
	"github.com/thomasgormley/dev-cli-go/internal/githubapi"
)

func handleHealth(config OpenCodeConfig) http.Handler {
	type response struct {
		Status       string `json:"status"`
		OpenCodeLive bool   `json:"opencodeLive"`
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encode(w, http.StatusOK, response{Status: "ok", OpenCodeLive: IsOpenCodeRunning(config.Host, config.Port)})
	})
}

func handlerAgentDispatch(ghClient *githubapi.Client, user string, opencodeConfig OpenCodeConfig) http.Handler {
	type request struct {
		URL string `json:"url"`
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

		prDetails, err := fetchPRDetails(r.Context(), ghClient, prInfo)
		if err != nil {
			log.Printf("fetchPRDetails failed: %v", err)
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

		opencodeClient, sessionID, err := createOpencodeSession(r.Context(), repo.Path(), opencodeConfig)
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
		processedThisRun := make(map[int64]bool)
		for _, c := range actionable {
			if isProcessed(c, user, processedThisRun) {
				log.Printf("comment %d already processed, skipping", c.ID)
				continue
			}
			processedThisRun[c.ID] = true

			if err := react(r.Context(), ghClient, c, prInfo); err != nil {
				log.Printf("comment %d: reaction failed: %v", c.ID, err)
				continue
			}

			promptText := prompt(c, prDetails)
			replyText, err := chat(r.Context(), opencodeClient, sessionID, promptText, repo.Path(), opencodeConfig)
			if err != nil {
				log.Printf("comment %d: chat failed: %v", c.ID, err)
				continue
			}

			commitMsg, err := chat(r.Context(), opencodeClient, sessionID, fmt.Sprintf("Summarize the following in less than 40 characters:\n\n%s", replyText), repo.Path(), opencodeConfig)
			if err != nil {
				commitMsg = "auto"
			}

			if err := commit(repo, branch, commitMsg); err != nil {
				log.Printf("comment %d: commit failed: %v", c.ID, err)
				continue
			}

			if err := repo.SyncToRemote(branch); err != nil {
				log.Printf("comment %d: sync to remote failed: %v", c.ID, err)
				continue
			}

			var commentURL string
			if c.IsReviewComment() {
				commentURL = fmt.Sprintf("https://github.com/%s/%s/pull/%d#discussion_r%d", prInfo.Owner, prInfo.Repo, prInfo.Number, c.ID)
			} else {
				commentURL = fmt.Sprintf("https://github.com/%s/%s/pull/%d#issuecomment-%d", prInfo.Owner, prInfo.Repo, prInfo.Number, c.ID)
			}
			metadata := fmt.Sprintf("<details><summary>👉 info</summary>\n\n| Key | Value |\n|-----|-------|\n| In reply to | [#%d](%s) |\n| Session | `%s` |\n\n</details>", c.ID, commentURL, sessionID)
			if err := comment(r.Context(), ghClient, c, prInfo, replyText+"\n\n"+metadata); err != nil {
				log.Printf("comment %d: comment failed: %v", c.ID, err)
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
		encode(w, http.StatusOK, response{SessionID: sessionID, Comments: actionable, AgentReplies: agentReplies, RepoPath: repo.Path(), Status: status})
	})
}

func handlerDebug(ghClient *githubapi.Client) http.Handler {
	type request struct {
		URL string `json:"url"`
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
			log.Printf("GetPRDetails failed: %v", err)
			encode(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		encode(w, http.StatusOK, data)
	})
}
