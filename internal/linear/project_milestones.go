package linear

import (
	"context"
	"errors"
	"fmt"

	"github.com/shurcooL/graphql"
)

type ProjectMilestone struct {
	ID         graphql.ID     `graphql:"id"`
	Name       graphql.String `graphql:"name"`
	TargetDate graphql.String `graphql:"targetDate"`
	Status     graphql.String `graphql:"status"`
	Project    Project        `graphql:"project"`
}

type ProjectMilestoneListRequest = Pagination

type ProjectMilestonePage struct {
	Items    []ProjectMilestone
	PageInfo PageInfo
}

// ProjectMilestoneFilter must keep Linear's GraphQL input type name because shurcooL/graphql
// derives variable declarations from Go type names.
type ProjectMilestoneFilter struct {
	Name *stringComparator `json:"name,omitempty"`
}

type projectMilestoneConnection struct {
	Nodes    []ProjectMilestone `graphql:"nodes"`
	PageInfo graphqlPageInfo    `graphql:"pageInfo"`
}

type projectMilestonesQuery struct {
	Project struct {
		ProjectMilestones projectMilestoneConnection `graphql:"projectMilestones(first: $first, after: $after)"`
	} `graphql:"project(id: $projectID)"`
}

type filteredProjectMilestonesQuery struct {
	Project struct {
		ProjectMilestones projectMilestoneConnection `graphql:"projectMilestones(first: $first, after: $after, filter: $filter)"`
	} `graphql:"project(id: $projectID)"`
}

func (c Client) ListProjectMilestones(
	ctx context.Context,
	projectID string,
	request ProjectMilestoneListRequest,
) (ProjectMilestonePage, error) {
	if request.Limit < 1 {
		return ProjectMilestonePage{}, errors.New("project milestone list limit must be greater than zero")
	}

	items := []ProjectMilestone{}
	cursor := request.Cursor
	pageInfo := PageInfo{}
	for len(items) < request.Limit {
		connection, err := c.queryProjectMilestones(
			ctx,
			projectID,
			Pagination{Limit: request.Limit - len(items), Cursor: cursor},
		)
		if err != nil {
			return ProjectMilestonePage{}, &APIError{Operation: "list project milestones", Err: err}
		}
		for _, milestone := range connection.Nodes {
			if len(items) < request.Limit {
				items = append(items, milestone)
			}
		}
		pageInfo = newPageInfo(connection.PageInfo)
		if !pageInfo.HasNextPage {
			break
		}
		cursor = pageInfo.NextCursor
	}
	return ProjectMilestonePage{Items: items, PageInfo: pageInfo}, nil
}

func (c Client) FindProjectMilestone(
	ctx context.Context,
	projectID string,
	selector string,
) (ProjectMilestone, bool, error) {
	if selector == "" {
		return ProjectMilestone{}, false, nil
	}

	matches, err := c.queryProjectMilestonesByFilter(
		ctx,
		projectID,
		ProjectMilestoneFilter{Name: exactComparator(selector)},
	)
	if err != nil {
		return ProjectMilestone{}, false, &APIError{Operation: "resolve project milestone", Err: err}
	}
	return selectProjectMilestone(selector, matches)
}

func (c Client) queryProjectMilestones(
	ctx context.Context,
	projectID string,
	pagination Pagination,
) (projectMilestoneConnection, error) {
	var query projectMilestonesQuery
	variables := map[string]any{
		"projectID": graphQLString(projectID),
		"first":     graphql.Int(pagination.Limit),
		"after":     newCursor(pagination.Cursor),
	}
	if err := c.client.Query(ctx, &query, variables); err != nil {
		return projectMilestoneConnection{}, err
	}
	return query.Project.ProjectMilestones, nil
}

func (c Client) queryProjectMilestonesByFilter(
	ctx context.Context,
	projectID string,
	filter ProjectMilestoneFilter,
) ([]ProjectMilestone, error) {
	items := []ProjectMilestone{}
	cursor := ""
	for {
		var query filteredProjectMilestonesQuery
		variables := map[string]any{
			"projectID": graphQLString(projectID),
			"first":     graphql.Int(50),
			"after":     newCursor(cursor),
			"filter":    filter,
		}
		if err := c.client.Query(ctx, &query, variables); err != nil {
			return nil, err
		}
		for _, milestone := range query.Project.ProjectMilestones.Nodes {
			items = append(items, milestone)
		}
		pageInfo := newPageInfo(query.Project.ProjectMilestones.PageInfo)
		if !pageInfo.HasNextPage {
			return items, nil
		}
		cursor = pageInfo.NextCursor
	}
}

func selectProjectMilestone(selector string, candidates []ProjectMilestone) (ProjectMilestone, bool, error) {
	switch len(candidates) {
	case 0:
		return ProjectMilestone{}, false, nil
	case 1:
		return candidates[0], true, nil
	default:
		return ProjectMilestone{}, false, fmt.Errorf("multiple project milestones match %q", selector)
	}
}
