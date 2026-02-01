package serve

import (
	"fmt"
	"log"
	"net/http"

	"github.com/thomasgormley/dev-cli-go/internal/git"
	"github.com/thomasgormley/dev-cli-go/internal/githubapi"
	"github.com/thomasgormley/dev-cli-go/internal/queuelib"
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

func handlerAgentDispatch(queue queuelib.Queue[agentDispatchJob], ghClient *githubapi.Client, user string, opencodeConfig OpenCodeConfig) http.Handler {
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

		sessionID, err := createOpencodeSession(r.Context(), repo.Path(), opencodeConfig)
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
