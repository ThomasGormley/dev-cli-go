package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type AgentResponse struct {
	SessionID    string       `json:"sessionId"`
	Comments     []Comment    `json:"comments"`
	AgentReplies []AgentReply `json:"agentReplies"`
	RepoPath     string       `json:"repoPath"`
	Status       string       `json:"status"`
}

type AgentReply struct {
	CommentID string `json:"commentId"`
	Reply     string `json:"reply"`
}

type DispatchRequest struct {
	URL string `json:"url"`
}

type Client struct {
	baseURL string
	http    *http.Client
}

func (c *Client) String() string {
	return c.baseURL
}

func NewClient(baseURL string) Client {
	return Client{
		baseURL: baseURL,
		http:    http.DefaultClient,
	}
}

func (c *Client) IsServerRunning(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/health", nil)
	if err != nil {
		return false
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

func (c *Client) DispatchAgent(ctx context.Context, prURL string) (*AgentResponse, error) {
	reqBody := DispatchRequest{URL: prURL}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("encoding body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/agent/dispatch", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dispatching to agent: %w", err)
	}

	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent dispatch failed: %s", respBody)
	}

	var result AgentResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}
