package linear

import (
	"context"
	"errors"
	"fmt"

	"github.com/shurcooL/graphql"
)

const relationTypeBlocks = "blocks"

type IssueRef struct {
	ID         graphql.ID     `graphql:"id"`
	Identifier graphql.String `graphql:"identifier"`
}

// IssueRelation is read from an issue's inverse relations, so Issue is the relation's source.
type IssueRelation struct {
	ID    graphql.ID     `graphql:"id"`
	Type  graphql.String `graphql:"type"`
	Issue IssueRef       `graphql:"issue"`
}

type IssueRelationConnection struct {
	Nodes    []IssueRelation `graphql:"nodes"`
	PageInfo graphqlPageInfo `graphql:"pageInfo"`
}

// BlockedBy returns the relations in which another issue blocks this one. It fails rather than
// return a partial list when Linear truncated the issue's inverse relations.
func (i Issue) BlockedBy() ([]IssueRelation, error) {
	if i.InverseRelations.PageInfo.HasNextPage {
		return nil, fmt.Errorf("issue %s has more than 50 inverse relations", i.Identifier)
	}
	blockers := []IssueRelation{}
	for _, relation := range i.InverseRelations.Nodes {
		if relation.Type == relationTypeBlocks {
			blockers = append(blockers, relation)
		}
	}
	return blockers, nil
}

// IssueRelationCreateInput must keep Linear's GraphQL input type name because shurcooL/graphql
// derives variable declarations from Go type names.
type IssueRelationCreateInput struct {
	IssueID        string `json:"issueId"`
	RelatedIssueID string `json:"relatedIssueId"`
	Type           string `json:"type"`
}

// NewBlockedByRelation records that blockerID blocks issueID. Linear has no "blocked by" type, so
// the blocker is the relation's source issue.
func NewBlockedByRelation(issueID, blockerID string) IssueRelationCreateInput {
	return IssueRelationCreateInput{IssueID: blockerID, RelatedIssueID: issueID, Type: relationTypeBlocks}
}

func (c Client) CreateIssueRelation(ctx context.Context, input IssueRelationCreateInput) error {
	var m struct {
		IssueRelationCreate struct {
			Success graphql.Boolean `graphql:"success"`
		} `graphql:"issueRelationCreate(input: $input)"`
	}
	if err := c.client.Mutate(ctx, &m, map[string]any{"input": input}); err != nil {
		return &APIError{Operation: "create issue relation", Err: err}
	}
	if !m.IssueRelationCreate.Success {
		return &APIError{Operation: "create issue relation", Err: errors.New("mutation was unsuccessful")}
	}
	return nil
}

func (c Client) DeleteIssueRelation(ctx context.Context, id string) error {
	var m struct {
		IssueRelationDelete struct {
			Success graphql.Boolean `graphql:"success"`
		} `graphql:"issueRelationDelete(id: $id)"`
	}
	if err := c.client.Mutate(ctx, &m, map[string]any{"id": graphql.String(id)}); err != nil {
		return &APIError{Operation: "delete issue relation", Err: err}
	}
	if !m.IssueRelationDelete.Success {
		return &APIError{Operation: "delete issue relation", Err: errors.New("mutation was unsuccessful")}
	}
	return nil
}
