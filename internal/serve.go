package cli

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/thomasgormley/dev-cli-go/internal/githubapi"
	"github.com/thomasgormley/dev-cli-go/internal/serve"
	"github.com/urfave/cli/v2"
)

func handleServe() cli.ActionFunc {

	return func(c *cli.Context) error {
		var host, port = c.String("host"), c.String("port")

		ghToken := os.Getenv("DEV_GITHUB_TOKEN")
		if ghToken == "" {
			return fmt.Errorf("DEV_GITHUB_TOKEN environment variable required")
		}
		ghClient := githubapi.NewClient(ghToken)

		srv := &http.Server{
			Addr: net.JoinHostPort(host, port),
			Handler: serve.Handle(serve.HandleOpts{
				GitHubUser:     os.Getenv("DEV_GITHUB_USER"),
				GitHubClient:   ghClient,
				AllowedOrigins: []string{"http://" + host, "https://" + host},
			}),
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
