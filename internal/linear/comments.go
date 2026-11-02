package linear

import (
	"context"
	"errors"

	"github.com/shurcooL/graphql"
)

type Comment struct {
	ID    graphql.ID     `graphql:"id"`
	URL   graphql.String `graphql:"url"`
	Body  graphql.String `graphql:"body"`
	Issue IssueRef       `graphql:"issue"`
}

// CommentCreateInput must keep Linear's GraphQL input type name because shurcooL/graphql
// derives variable declarations from Go type names.
type CommentCreateInput struct {
	IssueID string `json:"issueId"`
	Body    string `json:"body"`
}

func (c Client) CreateComment(ctx context.Context, input CommentCreateInput) (Comment, error) {
	var m struct {
		CommentCreate struct {
			Comment Comment         `graphql:"comment"`
			Success graphql.Boolean `graphql:"success"`
		} `graphql:"commentCreate(input: $input)"`
	}
	if err := c.client.Mutate(ctx, &m, map[string]any{"input": input}); err != nil {
		return Comment{}, &APIError{Operation: "create comment", Err: err}
	}
	if !m.CommentCreate.Success {
		return Comment{}, &APIError{Operation: "create comment", Err: errors.New("mutation was unsuccessful")}
	}
	return m.CommentCreate.Comment, nil
}
