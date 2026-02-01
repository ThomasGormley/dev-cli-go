package githubapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/go-github/v69/github"
	"golang.org/x/oauth2"
)

type Client struct {
	client *github.Client
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
	CreatedAt        string `json:"createdAt"`
	Path             string `json:"path"`
	Line             int    `json:"line"`
	StartLine        int    `json:"startLine"`
	OriginalLine     int    `json:"originalLine"`
	OriginalPosition int    `json:"originalPosition"`
	DiffHunk         string `json:"diffHunk"`
	Commit           struct {
		OID string `json:"oid"`
	} `json:"commit"`
	OriginalCommit struct {
		OID string `json:"oid"`
	} `json:"originalCommit"`
	Reactions struct {
		Nodes []struct {
			Content string `json:"content"`
			User    struct {
				Login string `json:"login"`
			}
		} `json:"nodes"`
	} `json:"reactions"`
}

type GraphQLPRResponse struct {
	Repository struct {
		PullRequest struct {
			Title       string `json:"title"`
			Body        string `json:"body"`
			HeadRefName string `json:"headRefName"`
			BaseRefName string `json:"baseRefName"`
			Additions   int    `json:"additions"`
			Deletions   int    `json:"deletions"`
			Commits     struct {
				TotalCount int `json:"totalCount"`
			} `json:"commits"`
			Author struct {
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
					Body     string `json:"body"`
					Comments struct {
						Nodes []GraphQLComment `json:"nodes"`
					} `json:"comments"`
				} `json:"nodes"`
			} `json:"reviews"`
			Files struct {
				Nodes []struct {
					Path       string `json:"path"`
					ChangeType string `json:"changeType"`
					Additions  int    `json:"additions"`
					Deletions  int    `json:"deletions"`
				} `json:"nodes"`
			} `json:"files"`
		} `json:"pullRequest"`
	} `json:"repository"`
}

func (c *Client) GetPRDetails(ctx context.Context, owner, repo string, number int) (*GraphQLPRResponse, error) {
	query := `query($owner: String!, $repo: String!, $number: Int!) {
		repository(owner: $owner, name: $repo) {
			pullRequest(number: $number) {
				title
				body
				headRefName
				baseRefName
				additions
				deletions
				commits(first: 1) {
					totalCount
				}
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
						body
						comments(first: 100) {
							nodes {
								databaseId
								body
								path
								line
								startLine
								originalLine
								originalPosition
								diffHunk
								commit { oid }
								originalCommit { oid }
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
				files(first: 100) {
					nodes {
						path
						changeType
						additions
						deletions
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

	payload := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL payload: %w", err)
	}

	httpReq, err := http.NewRequest("POST", "https://api.github.com/graphql", bytes.NewReader(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpClient := c.client.Client()
	resp, err := httpClient.Do(httpReq.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	type GraphQLError struct {
		Message string `json:"message"`
	}
	type GraphQLResponseWrapper struct {
		Data   GraphQLPRResponse `json:"data"`
		Errors []GraphQLError    `json:"errors,omitempty"`
	}

	var wrapper GraphQLResponseWrapper
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("failed to decode GraphQL response: %w", err)
	}

	if len(wrapper.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors: %v", wrapper.Errors)
	}

	return &wrapper.Data, nil
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
