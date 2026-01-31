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
	pollInterval time.Duration // Critical for future polling-based notification workflows
	mu           sync.RWMutex
}

type NotificationsResult struct {
	Notifications []*github.Notification
	Modified      bool
	PollInterval  time.Duration // Critical for future work - indicates when to poll next
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
	pollInterval := c.pollInterval
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
				PollInterval:  pollInterval,
			}, nil
		}
		return nil, err
	}

	c.mu.Lock()
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		c.lastModified = lm
	}
	// Critical for future work: persist poll interval for smart notification polling
	c.pollInterval = getPollInterval(resp)
	c.mu.Unlock()

	return &NotificationsResult{
		Notifications: notifications,
		Modified:      true,
		PollInterval:  pollInterval,
	}, nil
}

// getPollInterval extracts recommended poll interval from response headers.
// Critical for future work: enables smart, rate-limit-aware polling behavior.
func getPollInterval(resp *github.Response) time.Duration {
	if resp == nil || resp.Response == nil {
		return 60 * time.Second
	}
	if s := resp.Header.Get("X-Poll-Interval"); s != "" {
		if d, err := time.ParseDuration(s + "s"); err == nil {
			return d
		}
	}
	return 60 * time.Second
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

func (c *Client) CreateComment(ctx context.Context, owner, repo string, number int, body string) error {
	_, _, err := c.client.Issues.CreateComment(ctx, owner, repo, number, &github.IssueComment{
		Body: github.Ptr(body),
	})
	return err
}

func (c *Client) CreateReviewReply(ctx context.Context, owner, repo string, prNumber, commentID int64, body string) string {
	return ""
}

func (c *Client) CreateReaction(ctx context.Context, owner, repo string, commentID int64, reaction string) error {
	_, _, err := c.client.Reactions.CreateIssueCommentReaction(ctx, owner, repo, commentID, reaction)
	return err
}

func (c *Client) CreatePullRequestCommentReaction(ctx context.Context, owner, repo string, commentID int64, reaction string) error {
	_, _, err := c.client.Reactions.CreatePullRequestCommentReaction(ctx, owner, repo, commentID, reaction)
	return err
}

func (c *Client) CreateReviewCommentReply(ctx context.Context, owner, repo string, prNumber int, commentID int64, body string) error {
	url := fmt.Sprintf("repos/%s/%s/pulls/%d/comments/%d/replies", owner, repo, prNumber, commentID)
	req, err := c.client.NewRequest("POST", url, map[string]string{"body": body})
	if err != nil {
		return err
	}
	_, err = c.client.Do(ctx, req, nil)
	return err
}
