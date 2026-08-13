package linear

import (
	"context"
	"errors"
	"fmt"

	"github.com/shurcooL/graphql"
)

type Team struct {
	ID        graphql.ID     `graphql:"id"`
	Key       graphql.String `graphql:"key"`
	Name      graphql.String `graphql:"name"`
	RetiredAt graphql.String `graphql:"retiredAt"`
}

type TeamListRequest = Pagination

type TeamPage struct {
	Items    []Team
	PageInfo PageInfo
}

// TeamFilter must keep Linear's GraphQL input type name because shurcooL/graphql
// derives variable declarations from Go type names.
type TeamFilter struct {
	Key  *stringComparator `json:"key,omitempty"`
	Name *stringComparator `json:"name,omitempty"`
}

type teamConnection struct {
	Nodes    []Team          `graphql:"nodes"`
	PageInfo graphqlPageInfo `graphql:"pageInfo"`
}

type teamsQuery struct {
	Teams teamConnection `graphql:"teams(first: $first, after: $after)"`
}

type filteredTeamsQuery struct {
	Teams teamConnection `graphql:"teams(first: $first, after: $after, filter: $filter)"`
}

func (c Client) ListTeams(ctx context.Context, request TeamListRequest) (TeamPage, error) {
	if request.Limit < 1 {
		return TeamPage{}, errors.New("team list limit must be greater than zero")
	}

	items := []Team{}
	cursor := request.Cursor
	pageInfo := PageInfo{}
	for len(items) < request.Limit {
		connection, err := c.queryTeams(ctx, Pagination{Limit: request.Limit - len(items), Cursor: cursor})
		if err != nil {
			return TeamPage{}, &APIError{Operation: "list teams", Err: err}
		}
		for _, team := range connection.Nodes {
			if len(items) < request.Limit && isActiveTeam(team) {
				items = append(items, team)
			}
		}
		pageInfo = newPageInfo(connection.PageInfo)
		if !pageInfo.HasNextPage {
			break
		}
		cursor = pageInfo.NextCursor
	}
	return TeamPage{Items: items, PageInfo: pageInfo}, nil
}

func (c Client) FindTeam(ctx context.Context, selector string) (Team, bool, error) {
	if selector == "" {
		return Team{}, false, nil
	}

	keyMatches, err := c.queryTeamsByFilter(ctx, TeamFilter{Key: exactComparator(selector)})
	if err != nil {
		return Team{}, false, &APIError{Operation: "resolve team", Err: err}
	}
	if len(keyMatches) > 0 {
		return selectTeam(selector, keyMatches)
	}

	nameMatches, err := c.queryTeamsByFilter(ctx, TeamFilter{Name: exactComparator(selector)})
	if err != nil {
		return Team{}, false, &APIError{Operation: "resolve team", Err: err}
	}
	return selectTeam(selector, nameMatches)
}

func (c Client) queryTeams(ctx context.Context, pagination Pagination) (teamConnection, error) {
	var query teamsQuery
	variables := map[string]any{
		"first": graphql.Int(pagination.Limit),
		"after": newCursor(pagination.Cursor),
	}
	if err := c.client.Query(ctx, &query, variables); err != nil {
		return teamConnection{}, err
	}
	return query.Teams, nil
}

func (c Client) queryTeamProjects(
	ctx context.Context,
	teamID string,
	pagination Pagination,
) (projectConnection, error) {
	var query teamProjectsQuery
	variables := map[string]any{
		"teamID": graphQLString(teamID),
		"first":  graphql.Int(pagination.Limit),
		"after":  newCursor(pagination.Cursor),
	}
	if err := c.client.Query(ctx, &query, variables); err != nil {
		return projectConnection{}, err
	}
	return query.Team.Projects, nil
}

func (c Client) queryTeamsByFilter(ctx context.Context, filter TeamFilter) ([]Team, error) {
	items := []Team{}
	cursor := ""
	for {
		var query filteredTeamsQuery
		variables := map[string]any{
			"first":  graphql.Int(50),
			"after":  newCursor(cursor),
			"filter": filter,
		}
		if err := c.client.Query(ctx, &query, variables); err != nil {
			return nil, err
		}
		for _, team := range query.Teams.Nodes {
			if isActiveTeam(team) {
				items = append(items, team)
			}
		}
		pageInfo := newPageInfo(query.Teams.PageInfo)
		if !pageInfo.HasNextPage {
			return items, nil
		}
		cursor = pageInfo.NextCursor
	}
}

func selectTeam(selector string, candidates []Team) (Team, bool, error) {
	switch len(candidates) {
	case 0:
		return Team{}, false, nil
	case 1:
		return candidates[0], true, nil
	default:
		return Team{}, false, fmt.Errorf("multiple active teams match %q", selector)
	}
}

func isActiveTeam(team Team) bool {
	return team.ID != nil && team.RetiredAt == ""
}
