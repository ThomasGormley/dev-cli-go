package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/go-github/v69/github"
	"github.com/thomasgormley/dev-cli-go/internal/githubapi"
	"github.com/urfave/cli/v2"
)

func handleServe() cli.ActionFunc {
	mux := http.NewServeMux()
	forceTick := make(chan struct{}, 1)

	mux.HandleFunc("/api/force-poll", func(w http.ResponseWriter, r *http.Request) {
		select {
		case forceTick <- struct{}{}:
			w.WriteHeader(http.StatusAccepted)
			log.Printf("force poll triggered")
		default:
			w.WriteHeader(http.StatusTooManyRequests)
			log.Printf("force poll already pending")
		}
	})

	mux.HandleFunc("/api/agent/trigger", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST, OPTIONS")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var payload struct {
			URL       string `json:"url"`
			Timestamp string `json:"timestamp"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Printf("agent triggered with URL: %s", payload.URL)
		w.WriteHeader(http.StatusAccepted)
	})

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	return func(c *cli.Context) error {
		var host, port = c.String("host"), c.String("port")
		srv := &http.Server{
			Addr:    net.JoinHostPort(host, port),
			Handler: mux,
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

		go pollGitHubNotifications(c.Context, forceTick)

		wg.Wait()
		return nil
	}
}

func pollGitHubNotifications(ctx context.Context, forceTick chan struct{}) {
	log.Printf("polling github notifs")

	token := os.Getenv("DEV_GITHUB_TOKEN")
	if token == "" {
		log.Printf("DEV_GITHUB_TOKEN not set, skipping notifications polling")
		return
	}

	client := githubapi.NewClient(token)
	ticker := time.NewTicker(client.PollInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			result, err := client.ListNotifications(ctx)
			if err != nil {
				log.Printf("error fetching notifications: %s", err)
				continue
			}
			updateTicker(ticker, result.PollInterval)
			if result.Modified {
				logNotifications(result.Notifications)
			} else {
				log.Printf("no changes (304)")
			}
		case <-forceTick:
			log.Printf("force tick triggered")
			result, err := client.ListNotifications(ctx)
			if err != nil {
				log.Printf("error fetching notifications: %s", err)
				continue
			}
			updateTicker(ticker, result.PollInterval)
			logNotifications(result.Notifications)
		case <-ctx.Done():
			log.Printf("done")
			return
		}
	}
}

func updateTicker(ticker *time.Ticker, newInterval time.Duration) {
	ticker.Reset(newInterval)
}

func logNotifications(notifications []*github.Notification) {
	if len(notifications) == 0 {
		log.Printf("no unread notifications")
		return
	}
	log.Printf("found %d unread notifications", len(notifications))
	for _, n := range notifications {
		log.Printf("- %s: %s", n.GetSubject().GetType(), n.GetSubject().GetTitle())
	}
}
