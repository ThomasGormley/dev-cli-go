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

func createOpencodeSession(ctx context.Context, repoPath string, config OpenCodeConfig) (string, error) {
	openCodeURL := fmt.Sprintf("http://%s:%s", config.Host, config.Port)
	client := opencode.NewClient(option.WithBaseURL(openCodeURL))

	session, err := client.Session.New(ctx, opencode.SessionNewParams{
		Directory: opencode.String(repoPath),
	})
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	return session.ID, nil
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

func buildPRSystemPrompt(prInfo GitHubPR, prDetails PRDetails) string {
	var parts []string

	parts = append(parts, "Instructions:")
	parts = append(parts, "- You are helping with a GitHub pull request")
	parts = append(parts, "- Read each comment and execute the requested task")
	parts = append(parts, "- Commit changes with clear, concise messages")
	parts = append(parts, "- Reply to comments with your changes and include session metadata")
	parts = append(parts, "- Use bash to make changes, then commit and push")
	parts = append(parts, "")

	parts = append(parts, "PR Details:")
	parts = append(parts, fmt.Sprintf("- PR #%d: %s", prDetails.Number, prDetails.Title))
	parts = append(parts, fmt.Sprintf("- Author: %s", prDetails.Author))
	parts = append(parts, fmt.Sprintf("- Base: %s <- Head: %s", prDetails.BaseBranch, prDetails.HeadBranch))
	if prDetails.Body != "" {
		parts = append(parts, fmt.Sprintf("- Description: %s", prDetails.Body))
	}
	parts = append(parts, fmt.Sprintf("- Changes: +%d/-%d", prDetails.Additions, prDetails.Deletions))
	parts = append(parts, "")

	if len(prDetails.Files) > 0 {
		parts = append(parts, "Changed Files:")
		for _, f := range prDetails.Files {
			parts = append(parts, fmt.Sprintf("- %s (%s) +%d/-%d", f.Path, f.ChangeType, f.Additions, f.Deletions))
		}
		parts = append(parts, "")
	}

	parts = append(parts, "Previous Comments:")
	for _, c := range prDetails.Comments {
		var location string
		if c.IsReviewComment() {
			location = fmt.Sprintf(" (%s:%d)", c.FilePath, c.Line)
		}
		parts = append(parts, fmt.Sprintf("- %s at %s%s: %s", c.Author, c.CreatedAt, location, c.Body))
	}

	return strings.Join(parts, "\n")
}

func promptJob(ctx context.Context, client *opencode.Client, sessionID string, taskText string, systemPrompt string, repoPath string, config OpenCodeConfig) (string, error) {
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
		return "", fmt.Errorf("failed to prompt job: %w", err)
	}

	replyText := textFromRsp(rsp)
	if replyText == "" {
		return "", fmt.Errorf("failed to parse response")
	}

	return replyText, nil
}
