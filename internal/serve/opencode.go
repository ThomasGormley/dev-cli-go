package serve

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/sst/opencode-sdk-go"
)

func IsOpenCodeRunning(host, port string) bool {
	baseURL := fmt.Sprintf("http://%s:%s", host, port)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/health", nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

func waitForOpenCode(config OpenCodeConfig, timeout time.Duration) error {
	baseURL := fmt.Sprintf("http://%s:%s", config.Host, config.Port)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("OpenCode not reachable after %v at %s", timeout, baseURL)
		case <-ticker.C:
			if IsOpenCodeRunning(config.Host, config.Port) {
				return nil
			}
		}
	}
}

func StartOpenCode(ctx context.Context, config OpenCodeConfig, timeout time.Duration) (bool, func(), error) {
	baseURL := fmt.Sprintf("http://%s:%s", config.Host, config.Port)
	if IsOpenCodeRunning(config.Host, config.Port) {
		return false, func() {}, nil
	}

	cmd := exec.CommandContext(ctx, "opencode", "serve", "--hostname", config.Host, "--port", config.Port)
	if err := cmd.Start(); err != nil {
		return false, func() {}, fmt.Errorf("starting OpenCode: %w", err)
	}

	log.Printf("waiting for OpenCode to be ready at %s...", baseURL)
	if err := waitForOpenCode(config, timeout); err != nil {
		cmd.Process.Signal(os.Interrupt)
		cmd.Wait()
		return false, func() {}, fmt.Errorf("starting OpenCode: %w", err)
	}

	log.Printf("OpenCode is ready")
	closer := func() {
		if cmd.Process != nil {
			cmd.Process.Signal(os.Interrupt)
			cmd.Wait()
		}
	}
	return true, closer, nil
}

func createOpencodeSession(ctx context.Context, client opencode.Client, repoPath string, config OpenCodeConfig) (string, error) {

	session, err := client.Session.New(ctx, opencode.SessionNewParams{
		Directory: opencode.String(repoPath),
	})
	if err != nil {
		return "", fmt.Errorf("creating session: %w", err)
	}

	return session.ID, nil
}
