package linear

import (
	"github.com/shurcooL/graphql"
)

type PageInfo struct {
	HasNextPage bool
	NextCursor  string
}

type Pagination struct {
	Limit  int
	Cursor string
}

type graphqlPageInfo struct {
	HasNextPage graphql.Boolean `graphql:"hasNextPage"`
	EndCursor   graphql.String  `graphql:"endCursor"`
}

func newCursor(cursor string) *graphql.String {
	if cursor == "" {
		return nil
	}
	value := graphQLString(cursor)
	return &value
}

func newPageInfo(pageInfo graphqlPageInfo) PageInfo {
	return PageInfo{HasNextPage: bool(pageInfo.HasNextPage), NextCursor: string(pageInfo.EndCursor)}
}

type stringComparator struct {
	EqIgnoreCase *graphql.String `json:"eqIgnoreCase,omitempty"`
}

type booleanComparator struct {
	Eq *graphql.Boolean `json:"eq,omitempty"`
}

type idComparator struct {
	Eq *graphql.ID `json:"eq,omitempty"`
}

type numberComparator struct {
	Eq *graphql.Float `json:"eq,omitempty"`
}

func exactComparator(value string) *stringComparator {
	selector := graphQLString(value)
	return &stringComparator{EqIgnoreCase: &selector}
}

func exactIDComparator(value string) *idComparator {
	selector := graphQLID(value)
	return &idComparator{Eq: &selector}
}

func exactNumberComparator(value int) *numberComparator {
	selector := graphql.Float(value)
	return &numberComparator{Eq: &selector}
}
