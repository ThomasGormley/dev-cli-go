package serve

import (
	"log"
	"net/http"

	"github.com/thomasgormley/dev-cli-go/internal/githubapi"
)

func handleHealth(config OpenCodeConfig) http.Handler {
	type response struct {
		Status         string `json:"status"`
		OpenCodeLive   bool   `json:"opencodeLive"`
		OpenCodeURL    string `json:"opencodeUrl"`
		OpenCodeStatus string `json:"opencodeStatus"`
	}
	running := IsOpenCodeRunning(config.Host, config.Port)
	status := "stopped"
	if running {
		status = "running"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encode(w, http.StatusOK, response{
			Status:         "ok",
			OpenCodeLive:   running,
			OpenCodeURL:    "http://" + config.Host + ":" + config.Port,
			OpenCodeStatus: status,
		})
	})
}

func handlerDebug(ghClient githubapi.Client) http.Handler {
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
