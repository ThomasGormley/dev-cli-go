package githubapi

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/go-github/v69/github"
	"golang.org/x/oauth2"
)

type Client struct {
	client       *github.Client
	lastModified string
	pollInterval time.Duration
	mu           sync.RWMutex
}

type NotificationsResult struct {
	Notifications []*github.Notification
	Modified      bool
	PollInterval  time.Duration
}

func NewClient(token string) *Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	return &Client{
		client:       github.NewClient(tc),
		pollInterval: time.Minute,
	}
}

func (c *Client) ListNotifications(ctx context.Context) (*NotificationsResult, error) {
	opts := &github.NotificationListOptions{
		All:         false, // Only unread notifications
		ListOptions: github.ListOptions{PerPage: 50},
	}

	c.mu.RLock()
	lastMod := c.lastModified
	c.mu.RUnlock()

	req, err := c.client.NewRequest("GET", "notifications", opts)
	if err != nil {
		return nil, err
	}

	if lastMod != "" {
		req.Header.Set("If-Modified-Since", lastMod)
	}

	var notifications []*github.Notification
	resp, err := c.client.Do(ctx, req, &notifications)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotModified {
			return &NotificationsResult{
				Notifications: nil,
				Modified:      false,
				PollInterval:  c.getPollInterval(resp),
			}, nil
		}
		return nil, err
	}

	c.mu.Lock()
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		c.lastModified = lm
	}
	c.pollInterval = c.getPollInterval(resp)
	c.mu.Unlock()

	return &NotificationsResult{
		Notifications: notifications,
		Modified:      true,
		PollInterval:  c.getPollInterval(resp),
	}, nil
}

func (c *Client) getPollInterval(resp *github.Response) time.Duration {
	if pi := resp.Header.Get("X-Poll-Interval"); pi != "" {
		if seconds, err := time.ParseDuration(pi + "s"); err == nil {
			return seconds
		}
	}
	return time.Minute
}

func (c *Client) PollInterval() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pollInterval
}

func (c *Client) SearchMentions(ctx context.Context, username string, since time.Time) ([]*github.Issue, error) {
	query := fmt.Sprintf("@%s is:open updated:>%s", username, since.Format("2006-01-02"))
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 30},
	}
	result, _, err := c.client.Search.Issues(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	return result.Issues, nil
}

func (c *Client) GetIssueDetails(ctx context.Context, owner, repo string, number int) (*github.Issue, []*github.IssueComment, error) {
	issue, _, err := c.client.Issues.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, nil, err
	}

	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	comments, _, err := c.client.Issues.ListComments(ctx, owner, repo, number, opts)
	if err != nil {
		return issue, nil, err
	}

	return issue, comments, nil
}
