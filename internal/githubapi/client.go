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

type Reaction struct {
	Content string `json:"content"`
	User    string `json:"user"`
}

type GraphQLComment struct {
	DatabaseID int64  `json:"databaseId"`
	Body       string `json:"body"`
	Author     struct {
		Login string `json:"login"`
	} `json:"author"`
	CreatedAt    string     `json:"createdAt"`
	Reactions    []Reaction `json:"reactions"`
	IsReview     bool       `json:"-"`
	DiffHunk     string     `json:"-"`
	Path         string     `json:"path"`
	Line         int        `json:"line"`
	StartLine    int        `json:"startLine"`
	OriginalLine int        `json:"originalLine"`
}

type GraphQLPRResponse struct {
	Repository struct {
		PullRequest struct {
			Title       string `json:"title"`
			HeadRefName string `json:"headRefName"`
			Author      struct {
				Login string `json:"login"`
			} `json:"author"`
			Comments struct {
				Nodes []GraphQLComment `json:"nodes"`
			} `json:"comments"`
			Reviews struct {
				Nodes []struct {
					Author struct {
						Login string `json:"login"`
					} `json:"author"`
					Comments struct {
						Nodes []GraphQLComment `json:"nodes"`
					} `json:"comments"`
				} `json:"nodes"`
			} `json:"reviews"`
		} `json:"pullRequest"`
	} `json:"repository"`
}

func (c *Client) GetPRDetails(ctx context.Context, owner, repo string, number int) (*GraphQLPRResponse, error) {
	query := `query($owner: String!, $repo: String!, $number: Int!) {
		repository(owner: $owner, name: $repo) {
			pullRequest(number: $number) {
				title
				headRefName
				author { login }
				comments(first: 100) {
					nodes {
						databaseId
						body
						author { login }
						createdAt
						reactions(first: 100) {
							nodes {
								content
								user { login }
							}
						}
					}
				}
				reviews(first: 100) {
					nodes {
						author { login }
						comments(first: 100) {
							nodes {
								databaseId
								body
								path
								line
								startLine
								originalLine
								author { login }
								createdAt
								reactions(first: 100) {
									nodes {
										content
										user { login }
									}
								}
							}
						}
					}
				}
			}
		}
	}`

	variables := map[string]interface{}{
		"owner":  owner,
		"repo":   repo,
		"number": number,
	}

	body := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}

	req, err := c.client.NewRequest("POST", "graphql", body)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data GraphQLPRResponse `json:"data"`
	}
	_, err = c.client.Do(ctx, req, &resp)
	return &resp.Data, err
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
