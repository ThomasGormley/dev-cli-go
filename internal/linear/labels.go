package linear

import (
	"context"
	"errors"
	"fmt"

	"github.com/shurcooL/graphql"
)

type Label struct {
	ID      graphql.ID      `graphql:"id"`
	Name    graphql.String  `graphql:"name"`
	IsGroup graphql.Boolean `graphql:"isGroup"`
	Team    Team            `graphql:"team"`
}

type LabelListRequest = Pagination

type LabelPage struct {
	Items    []Label
	PageInfo PageInfo
}

// IssueLabelFilter must keep Linear's GraphQL input type name because shurcooL/graphql
// derives variable declarations from Go type names.
type IssueLabelFilter struct {
	Name *stringComparator `json:"name,omitempty"`
}

type LabelConnection struct {
	Nodes    []Label         `graphql:"nodes"`
	PageInfo graphqlPageInfo `graphql:"pageInfo"`
}

type labelsQuery struct {
	IssueLabels LabelConnection `graphql:"issueLabels(first: $first, after: $after)"`
}

type filteredLabelsQuery struct {
	IssueLabels LabelConnection `graphql:"issueLabels(first: $first, after: $after, filter: $filter)"`
}

func (c Client) ListLabels(ctx context.Context, teamID string, request LabelListRequest) (LabelPage, error) {
	if teamID == "" {
		return LabelPage{}, errors.New("label list team ID required")
	}
	if request.Limit < 1 {
		return LabelPage{}, errors.New("label list limit must be greater than zero")
	}

	items := []Label{}
	cursor := request.Cursor
	pageInfo := PageInfo{}
	for len(items) < request.Limit {
		connection, err := c.queryLabels(ctx, Pagination{Limit: request.Limit - len(items), Cursor: cursor})
		if err != nil {
			return LabelPage{}, &APIError{Operation: "list labels", Err: err}
		}
		for _, label := range connection.Nodes {
			if len(items) < request.Limit && isApplicableLabel(label, teamID) {
				items = append(items, label)
			}
		}
		pageInfo = newPageInfo(connection.PageInfo)
		if !pageInfo.HasNextPage {
			break
		}
		cursor = pageInfo.NextCursor
	}
	return LabelPage{Items: items, PageInfo: pageInfo}, nil
}

func (c Client) FindLabel(ctx context.Context, teamID, selector string) (Label, bool, error) {
	if teamID == "" || selector == "" {
		return Label{}, false, nil
	}
	nameMatches, err := c.queryLabelsByFilter(ctx, teamID, IssueLabelFilter{Name: exactComparator(selector)})
	if err != nil {
		return Label{}, false, &APIError{Operation: "resolve label", Err: err}
	}
	return selectLabel(selector, nameMatches)
}

func (c Client) queryLabels(ctx context.Context, pagination Pagination) (LabelConnection, error) {
	var query labelsQuery
	variables := map[string]any{
		"first": graphql.Int(pagination.Limit),
		"after": newCursor(pagination.Cursor),
	}
	if err := c.client.Query(ctx, &query, variables); err != nil {
		return LabelConnection{}, err
	}
	return query.IssueLabels, nil
}

func (c Client) queryLabelsByFilter(
	ctx context.Context,
	teamID string,
	filter IssueLabelFilter,
) ([]Label, error) {
	items := []Label{}
	cursor := ""
	for {
		var query filteredLabelsQuery
		variables := map[string]any{
			"first":  graphql.Int(50),
			"after":  newCursor(cursor),
			"filter": filter,
		}
		if err := c.client.Query(ctx, &query, variables); err != nil {
			return nil, err
		}
		for _, label := range query.IssueLabels.Nodes {
			if isApplicableLabel(label, teamID) {
				items = append(items, label)
			}
		}
		pageInfo := newPageInfo(query.IssueLabels.PageInfo)
		if !pageInfo.HasNextPage {
			return items, nil
		}
		cursor = pageInfo.NextCursor
	}
}

func selectLabel(selector string, candidates []Label) (Label, bool, error) {
	switch len(candidates) {
	case 0:
		return Label{}, false, nil
	case 1:
		if candidates[0].IsGroup {
			return Label{}, false, fmt.Errorf("label group %q cannot be applied to an issue", selector)
		}
		return candidates[0], true, nil
	default:
		return Label{}, false, fmt.Errorf("multiple applicable labels match %q", selector)
	}
}

func isApplicableLabel(label Label, teamID string) bool {
	if label.ID == nil {
		return false
	}
	if label.Team.ID == nil {
		return true
	}
	labelTeamID, ok := graphQLIDString(label.Team.ID)
	return ok && labelTeamID == teamID
}
