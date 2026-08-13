package linear

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/shurcooL/graphql"
)

type User struct {
	ID          graphql.ID      `graphql:"id"`
	Name        graphql.String  `graphql:"name"`
	DisplayName graphql.String  `graphql:"displayName"`
	Email       graphql.String  `graphql:"email"`
	Active      graphql.Boolean `graphql:"active"`
}

type UserListRequest = Pagination

type UserPage struct {
	Items    []User
	PageInfo PageInfo
}

// UserFilter must keep Linear's GraphQL input type name because shurcooL/graphql
// derives variable declarations from Go type names.
type UserFilter struct {
	Active      *booleanComparator `json:"active,omitempty"`
	ID          *idComparator      `json:"id,omitempty"`
	Email       *stringComparator  `json:"email,omitempty"`
	Name        *stringComparator  `json:"name,omitempty"`
	DisplayName *stringComparator  `json:"displayName,omitempty"`
}

type userConnection struct {
	Nodes    []User          `graphql:"nodes"`
	PageInfo graphqlPageInfo `graphql:"pageInfo"`
}

type teamMembersQuery struct {
	Team struct {
		Members userConnection `graphql:"members(first: $first, after: $after, filter: $filter)"`
	} `graphql:"team(id: $teamID)"`
}

func (c Client) ListUsers(ctx context.Context, teamID string, request UserListRequest) (UserPage, error) {
	if request.Limit < 1 {
		return UserPage{}, errors.New("user list limit must be greater than zero")
	}

	items := []User{}
	cursor := request.Cursor
	pageInfo := PageInfo{}
	for len(items) < request.Limit {
		connection, err := c.queryUsers(
			ctx,
			teamID,
			Pagination{Limit: request.Limit - len(items), Cursor: cursor},
			activeUserFilter(),
		)
		if err != nil {
			return UserPage{}, &APIError{Operation: "list users", Err: err}
		}
		for _, user := range connection.Nodes {
			if len(items) < request.Limit && isActiveUser(user) {
				items = append(items, user)
			}
		}
		pageInfo = newPageInfo(connection.PageInfo)
		if !pageInfo.HasNextPage {
			break
		}
		cursor = pageInfo.NextCursor
	}
	return UserPage{Items: items, PageInfo: pageInfo}, nil
}

func (c Client) FindAssignee(ctx context.Context, teamID, selector string) (User, bool, error) {
	if selector == "" {
		return User{}, false, nil
	}
	if strings.EqualFold(selector, "me") {
		viewer, err := c.Viewer(ctx)
		if err != nil {
			return User{}, false, &APIError{Operation: "resolve assignee", Err: err}
		}
		viewerID, ok := graphQLIDString(viewer.ID)
		if !ok {
			return User{}, false, nil
		}
		return c.resolveUserByFilter(ctx, teamID, selector, UserFilter{ID: exactIDComparator(viewerID)})
	}
	if strings.Contains(selector, "@") {
		return c.resolveUserByFilter(ctx, teamID, selector, UserFilter{Email: exactComparator(selector)})
	}

	nameMatches, err := c.queryUsersByFilter(ctx, teamID, UserFilter{Name: exactComparator(selector)})
	if err != nil {
		return User{}, false, &APIError{Operation: "resolve assignee", Err: err}
	}
	displayNameMatches, err := c.queryUsersByFilter(ctx, teamID, UserFilter{DisplayName: exactComparator(selector)})
	if err != nil {
		return User{}, false, &APIError{Operation: "resolve assignee", Err: err}
	}
	return selectUser(selector, uniqueUsers(nameMatches, displayNameMatches))
}

func (c Client) queryUsers(
	ctx context.Context,
	teamID string,
	pagination Pagination,
	filter UserFilter,
) (userConnection, error) {
	var query teamMembersQuery
	variables := map[string]any{
		"teamID": graphQLString(teamID),
		"first":  graphql.Int(pagination.Limit),
		"after":  newCursor(pagination.Cursor),
		"filter": filter,
	}
	if err := c.client.Query(ctx, &query, variables); err != nil {
		return userConnection{}, err
	}
	return query.Team.Members, nil
}

func (c Client) queryUsersByFilter(ctx context.Context, teamID string, filter UserFilter) ([]User, error) {
	filter.Active = activeUserFilter().Active
	items := []User{}
	cursor := ""
	for {
		connection, err := c.queryUsers(ctx, teamID, Pagination{Limit: 50, Cursor: cursor}, filter)
		if err != nil {
			return nil, err
		}
		for _, user := range connection.Nodes {
			if isActiveUser(user) {
				items = append(items, user)
			}
		}
		pageInfo := newPageInfo(connection.PageInfo)
		if !pageInfo.HasNextPage {
			return items, nil
		}
		cursor = pageInfo.NextCursor
	}
}

func (c Client) resolveUserByFilter(ctx context.Context, teamID, selector string, filter UserFilter) (User, bool, error) {
	matches, err := c.queryUsersByFilter(ctx, teamID, filter)
	if err != nil {
		return User{}, false, &APIError{Operation: "resolve assignee", Err: err}
	}
	return selectUser(selector, matches)
}

func selectUser(selector string, candidates []User) (User, bool, error) {
	switch len(candidates) {
	case 0:
		return User{}, false, nil
	case 1:
		return candidates[0], true, nil
	default:
		return User{}, false, fmt.Errorf("multiple active team members match %q", selector)
	}
}

func uniqueUsers(groups ...[]User) []User {
	users := []User{}
	seen := map[string]bool{}
	for _, group := range groups {
		for _, user := range group {
			id, ok := graphQLIDString(user.ID)
			if !ok || seen[id] {
				continue
			}
			seen[id] = true
			users = append(users, user)
		}
	}
	return users
}

func activeUserFilter() UserFilter {
	active := graphql.Boolean(true)
	return UserFilter{Active: &booleanComparator{Eq: &active}}
}

func isActiveUser(user User) bool {
	return user.ID != nil && bool(user.Active)
}

// Query struct for getting current user
type viewerQuery struct {
	Viewer User `graphql:"viewer"`
}

// RPC-style method to get current user
func (c Client) Viewer(ctx context.Context) (User, error) {
	var q viewerQuery
	err := c.client.Query(ctx, &q, nil)
	if err != nil {
		return User{}, err
	}
	return q.Viewer, nil
}
