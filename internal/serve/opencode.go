package serve

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode-sdk-go/option"
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
		return false, func() {}, fmt.Errorf("failed to start OpenCode: %w", err)
	}

	log.Printf("waiting for OpenCode to be ready at %s...", baseURL)
	if err := waitForOpenCode(config, timeout); err != nil {
		cmd.Process.Signal(os.Interrupt)
		cmd.Wait()
		return false, func() {}, fmt.Errorf("OpenCode failed to start: %w", err)
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

func createOpencodeSession(ctx context.Context, repoPath string, config OpenCodeConfig) (*opencode.Client, string, error) {
	openCodeURL := fmt.Sprintf("http://%s:%s", config.Host, config.Port)
	client := opencode.NewClient(option.WithBaseURL(openCodeURL))
	session, err := client.Session.New(ctx, opencode.SessionNewParams{
		Directory: opencode.String(repoPath),
	})
	if err != nil {
		return nil, "", err
	}
	return client, session.ID, nil
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

func chat(ctx context.Context, client *opencode.Client, sessionID string, text string, directory string, config OpenCodeConfig) (string, error) {
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
				ProviderID: opencode.String(config.Provider),
				ModelID:    opencode.String(config.Model),
			}),
			System: opencode.String("You are helping with a GitHub pull request. Follow instructions carefully.\n\nYou have bash access. When I ask you to create Linear tickets or perform actions, EXECUTE the commands directly using bash.\n\nLinear commands:\n- dev linear create --title \"...\" --description \"$(cat <<'EOF'\nmulti-line\ncontent\nEOF\n)\"\n- dev linear get <issue-id>\n- dev linear update <issue-id> --title \"...\" --description \"$(cat <<'EOF'\ncontent\nEOF\n)\"\n\nAll dev linear commands return JSON with `id` and `url` fields. IMPORTANT: After executing a command, PARSE the JSON output and INCLUDE the URL in your reply.\n\nExample:\nUser: Create a ticket for X\nYou: [execute command, parse JSON]\nTicket created: THO-123\nhttps://linear.app/issue/THO-123\n\nUse $(cat <<'EOF'...EOF) for multi-line descriptions."),
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
