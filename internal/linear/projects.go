package linear

import (
	"context"
	"errors"
	"fmt"

	"github.com/shurcooL/graphql"
)

type Project struct {
	ID         graphql.ID     `graphql:"id"`
	Name       graphql.String `graphql:"name"`
	SlugID     graphql.String `graphql:"slugId"`
	ArchivedAt graphql.String `graphql:"archivedAt"`
}

type ProjectListRequest = Pagination

type ProjectPage struct {
	Items    []Project
	PageInfo PageInfo
}

// ProjectFilter must keep Linear's GraphQL input type name because shurcooL/graphql
// derives variable declarations from Go type names.
type ProjectFilter struct {
	Name   *stringComparator `json:"name,omitempty"`
	SlugID *stringComparator `json:"slugId,omitempty"`
}

type projectConnection struct {
	Nodes    []Project       `graphql:"nodes"`
	PageInfo graphqlPageInfo `graphql:"pageInfo"`
}

type teamProjectsQuery struct {
	Team struct {
		Projects projectConnection `graphql:"projects(first: $first, after: $after)"`
	} `graphql:"team(id: $teamID)"`
}

type filteredTeamProjectsQuery struct {
	Team struct {
		Projects projectConnection `graphql:"projects(first: $first, after: $after, filter: $filter)"`
	} `graphql:"team(id: $teamID)"`
}

func (c Client) ListProjects(ctx context.Context, teamID string, request ProjectListRequest) (ProjectPage, error) {
	if request.Limit < 1 {
		return ProjectPage{}, errors.New("project list limit must be greater than zero")
	}

	items := []Project{}
	cursor := request.Cursor
	pageInfo := PageInfo{}
	for len(items) < request.Limit {
		connection, err := c.queryTeamProjects(
			ctx,
			teamID,
			Pagination{Limit: request.Limit - len(items), Cursor: cursor},
		)
		if err != nil {
			return ProjectPage{}, &APIError{Operation: "list projects", Err: err}
		}
		for _, project := range connection.Nodes {
			if len(items) < request.Limit && isActiveProject(project) {
				items = append(items, project)
			}
		}
		pageInfo = newPageInfo(connection.PageInfo)
		if !pageInfo.HasNextPage {
			break
		}
		cursor = pageInfo.NextCursor
	}
	return ProjectPage{Items: items, PageInfo: pageInfo}, nil
}

func (c Client) FindProject(ctx context.Context, teamID string, selector string) (Project, bool, error) {
	if selector == "" {
		return Project{}, false, nil
	}

	matches, err := c.queryProjectsByFilter(ctx, teamID, ProjectFilter{SlugID: exactComparator(selector)})
	if err != nil {
		return Project{}, false, &APIError{Operation: "resolve project", Err: err}
	}
	if len(matches) > 0 {
		return selectProject(selector, matches)
	}

	matches, err = c.queryProjectsByFilter(ctx, teamID, ProjectFilter{Name: exactComparator(selector)})
	if err != nil {
		return Project{}, false, &APIError{Operation: "resolve project", Err: err}
	}
	return selectProject(selector, matches)
}

func (c Client) queryProjectsByFilter(
	ctx context.Context,
	teamID string,
	filter ProjectFilter,
) ([]Project, error) {
	items := []Project{}
	cursor := ""
	for {
		var query filteredTeamProjectsQuery
		variables := map[string]any{
			"teamID": graphQLString(teamID),
			"first":  graphql.Int(50),
			"after":  newCursor(cursor),
			"filter": filter,
		}
		if err := c.client.Query(ctx, &query, variables); err != nil {
			return nil, err
		}
		for _, project := range query.Team.Projects.Nodes {
			if isActiveProject(project) {
				items = append(items, project)
			}
		}
		pageInfo := newPageInfo(query.Team.Projects.PageInfo)
		if !pageInfo.HasNextPage {
			return items, nil
		}
		cursor = pageInfo.NextCursor
	}
}

func selectProject(selector string, candidates []Project) (Project, bool, error) {
	switch len(candidates) {
	case 0:
		return Project{}, false, nil
	case 1:
		return candidates[0], true, nil
	default:
		return Project{}, false, fmt.Errorf("multiple active projects match %q", selector)
	}
}

func isActiveProject(project Project) bool {
	return project.ID != nil && project.ArchivedAt == ""
}
