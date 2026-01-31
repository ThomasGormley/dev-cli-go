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
	mu           sync.RWMutex
}

type NotificationsResult struct {
	Notifications []*github.Notification
	Modified      bool
}

func NewClient(token string) *Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	return &Client{
		client: github.NewClient(tc),
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
			}, nil
		}
		return nil, err
	}

	c.mu.Lock()
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		c.lastModified = lm
	}
	c.mu.Unlock()

	return &NotificationsResult{
		Notifications: notifications,
		Modified:      true,
	}, nil
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

func (c *Client) GetPullRequestDetails(ctx context.Context, owner, repo string, number int) (*github.PullRequest, []*github.IssueComment, []*github.PullRequestComment, error) {
	pr, _, err := c.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, nil, nil, err
	}

	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	issueComments, _, err := c.client.Issues.ListComments(ctx, owner, repo, number, opts)
	if err != nil {
		return pr, nil, nil, err
	}

	prOpts := &github.PullRequestListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	reviewComments, _, err := c.client.PullRequests.ListComments(ctx, owner, repo, number, prOpts)
	if err != nil {
		return pr, issueComments, nil, err
	}

	return pr, issueComments, reviewComments, nil
}
