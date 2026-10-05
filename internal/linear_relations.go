package cli

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/urfave/cli/v2"

	linear "github.com/thomasgormley/dev-cli-go/internal/linear"
)

type linearIssueRefOutput struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
}

type linearParentChange struct {
	Selector string
	Clear    bool
}

func (c linearParentChange) IsSet() bool {
	return c.Selector != "" || c.Clear
}

type linearBlockedByChange struct {
	Add    []string
	Remove []string
}

func (c linearBlockedByChange) IsSet() bool {
	return len(c.Add) > 0 || len(c.Remove) > 0
}

type linearLinkChange struct {
	Parent    linearParentChange
	BlockedBy linearBlockedByChange
}

func (c linearLinkChange) IsSet() bool {
	return c.Parent.IsSet() || c.BlockedBy.IsSet()
}

type linearResolvedLinkChange struct {
	Parent *linearIssueRefOutput
	Plan   linearBlockedByPlan
}

type linearRelationCreate struct {
	Blocker linearIssueRefOutput
	Input   linear.IssueRelationCreateInput
}

type linearBlockedByLink struct {
	Blocker    linearIssueRefOutput
	RelationID string
}

type linearBlockedByPlan struct {
	Create []linearRelationCreate
	Delete []linearBlockedByLink
}

func (p linearBlockedByPlan) IsEmpty() bool {
	return len(p.Create) == 0 && len(p.Delete) == 0
}

type linearRelationsDryRunOutput struct {
	Create []linear.IssueRelationCreateInput  `json:"create"`
	Delete []linearRelationDeleteDryRunOutput `json:"delete"`
}

type linearRelationDeleteDryRunOutput struct {
	ID string `json:"id"`
}

func linearLinkUpdate(c *cli.Context, issueID string) (linearLinkChange, error) {
	parent, err := linearParentUpdate(c, issueID)
	if err != nil {
		return linearLinkChange{}, err
	}
	blockedBy, err := linearBlockedByUpdate(c, issueID)
	if err != nil {
		return linearLinkChange{}, err
	}
	return linearLinkChange{Parent: parent, BlockedBy: blockedBy}, nil
}

func linearParentUpdate(c *cli.Context, issueID string) (linearParentChange, error) {
	hasParent := c.IsSet("parent")
	clearParent := c.Bool("clear-parent")
	if hasParent && clearParent {
		return linearParentChange{}, invalidLinearArguments("--parent and --clear-parent cannot be used together")
	}
	if !hasParent {
		return linearParentChange{Clear: clearParent}, nil
	}
	selector := c.String("parent")
	if selector == "" {
		return linearParentChange{}, invalidLinearArguments("--parent cannot be empty")
	}
	if sameLinearIssueSelector(selector, issueID) {
		return linearParentChange{}, invalidLinearArguments("an issue cannot be its own parent")
	}
	return linearParentChange{Selector: selector}, nil
}

func linearBlockedByUpdate(c *cli.Context, issueID string) (linearBlockedByChange, error) {
	change := linearBlockedByChange{Add: c.StringSlice("add-blocked-by"), Remove: c.StringSlice("remove-blocked-by")}
	for _, selector := range slices.Concat(change.Add, change.Remove) {
		if selector == "" {
			return linearBlockedByChange{}, invalidLinearArguments("blocked-by selectors cannot be empty")
		}
		if sameLinearIssueSelector(selector, issueID) {
			return linearBlockedByChange{}, invalidLinearArguments("an issue cannot be blocked by itself")
		}
	}
	for _, added := range change.Add {
		for _, removed := range change.Remove {
			if sameLinearIssueSelector(added, removed) {
				return linearBlockedByChange{}, invalidLinearArguments(
					"the same blocker cannot be added and removed in one command",
				)
			}
		}
	}
	return change, nil
}

// sameLinearIssueSelector compares identifiers before any lookup; Linear matches team keys
// case-insensitively.
func sameLinearIssueSelector(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func resolveLinearParent(c *cli.Context, client linear.Clienter) (*linearIssueRefOutput, error) {
	if !c.IsSet("parent") {
		return nil, nil
	}
	selector := c.String("parent")
	if selector == "" {
		return nil, invalidLinearArguments("--parent cannot be empty")
	}
	parent, err := resolveLinearIssueRef(c.Context, client, selector)
	if err != nil {
		return nil, err
	}
	return &parent, nil
}

func resolveLinearUpdateLinks(
	ctx context.Context,
	client linear.Clienter,
	issueID string,
	input linear.UpdateIssueRequest,
	change linearLinkChange,
) (linear.UpdateIssueRequest, linearResolvedLinkChange, error) {
	resolved := linearResolvedLinkChange{}
	input.ClearParent = change.Parent.Clear
	if change.Parent.Selector != "" {
		parent, err := resolveLinearIssueRef(ctx, client, change.Parent.Selector)
		if err != nil {
			return linear.UpdateIssueRequest{}, linearResolvedLinkChange{}, err
		}
		input.ParentID = parent.ID
		input.SetParent = true
		resolved.Parent = &parent
	}
	if !change.BlockedBy.IsSet() {
		return input, resolved, nil
	}
	issue, err := findLinearIssue(ctx, client, issueID)
	if err != nil {
		return linear.UpdateIssueRequest{}, linearResolvedLinkChange{}, err
	}
	add, err := resolveLinearIssueRefs(ctx, client, change.BlockedBy.Add)
	if err != nil {
		return linear.UpdateIssueRequest{}, linearResolvedLinkChange{}, err
	}
	remove, err := resolveLinearIssueRefs(ctx, client, change.BlockedBy.Remove)
	if err != nil {
		return linear.UpdateIssueRequest{}, linearResolvedLinkChange{}, err
	}
	if resolved.Plan, err = planLinearBlockedBy(issue, add, remove); err != nil {
		return linear.UpdateIssueRequest{}, linearResolvedLinkChange{}, linearAPIError("invalid issue response", err)
	}
	return input, resolved, nil
}

// planLinearBlockedBy skips blockers that are already linked and removals that are not, so a
// re-run only writes what is still missing.
func planLinearBlockedBy(issue linear.Issue, add, remove []linearIssueRefOutput) (linearBlockedByPlan, error) {
	issueID, ok := linearGraphQLIDString(issue.ID)
	if !ok {
		return linearBlockedByPlan{}, fmt.Errorf("issue ID is not a string: %T", issue.ID)
	}
	relations, err := issue.BlockedBy()
	if err != nil {
		return linearBlockedByPlan{}, err
	}
	existing := make([]linearBlockedByLink, 0, len(relations))
	for _, relation := range relations {
		blocker, err := newLinearIssueRefOutput(relation.Issue)
		if err != nil {
			return linearBlockedByPlan{}, err
		}
		relationID, ok := linearGraphQLIDString(relation.ID)
		if !ok {
			return linearBlockedByPlan{}, fmt.Errorf("relation ID is not a string: %T", relation.ID)
		}
		existing = append(existing, linearBlockedByLink{Blocker: blocker, RelationID: relationID})
	}

	plan := linearBlockedByPlan{Create: []linearRelationCreate{}, Delete: []linearBlockedByLink{}}
	for _, blocker := range add {
		if !slices.ContainsFunc(existing, func(l linearBlockedByLink) bool { return l.Blocker.ID == blocker.ID }) {
			plan.Create = append(plan.Create, linearRelationCreate{
				Blocker: blocker,
				Input:   linear.NewBlockedByRelation(issueID, blocker.ID),
			})
		}
	}
	for _, relation := range existing {
		if slices.ContainsFunc(remove, func(r linearIssueRefOutput) bool { return r.ID == relation.Blocker.ID }) {
			plan.Delete = append(plan.Delete, relation)
		}
	}
	return plan, nil
}

// applyLinearUpdate writes field changes before links and stops at the first failure. Every step
// is safe to repeat, so re-running the same command finishes the job.
func applyLinearUpdate(
	ctx context.Context,
	client linear.Clienter,
	issueID string,
	input linear.UpdateIssueRequest,
	plan linearBlockedByPlan,
) (linear.Issue, error) {
	if hasLinearUpdate(input) {
		issue, err := client.UpdateIssue(ctx, issueID, input)
		if err != nil {
			return linear.Issue{}, linearAPIError("failed to update issue", err)
		}
		if plan.IsEmpty() {
			return issue, nil
		}
	}
	for _, relation := range plan.Delete {
		if err := client.DeleteIssueRelation(ctx, relation.RelationID); err != nil {
			return linear.Issue{}, linearAPIError(
				fmt.Sprintf("failed to remove blocked-by %s", relation.Blocker.Identifier), err,
			)
		}
	}
	for _, relation := range plan.Create {
		if err := client.CreateIssueRelation(ctx, relation.Input); err != nil {
			return linear.Issue{}, linearAPIError(
				fmt.Sprintf("failed to add blocked-by %s", relation.Blocker.Identifier), err,
			)
		}
	}
	return findLinearIssue(ctx, client, issueID)
}

func newLinearRelationsDryRunOutput(plan linearBlockedByPlan) linearRelationsDryRunOutput {
	output := linearRelationsDryRunOutput{
		Create: make([]linear.IssueRelationCreateInput, 0, len(plan.Create)),
		Delete: make([]linearRelationDeleteDryRunOutput, 0, len(plan.Delete)),
	}
	for _, relation := range plan.Create {
		output.Create = append(output.Create, relation.Input)
	}
	for _, relation := range plan.Delete {
		output.Delete = append(output.Delete, linearRelationDeleteDryRunOutput{ID: relation.RelationID})
	}
	return output
}

func resolveLinearIssueRefs(
	ctx context.Context,
	client linear.Clienter,
	selectors []string,
) ([]linearIssueRefOutput, error) {
	refs := make([]linearIssueRefOutput, 0, len(selectors))
	for _, selector := range selectors {
		ref, err := resolveLinearIssueRef(ctx, client, selector)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(refs, ref) {
			refs = append(refs, ref)
		}
	}
	return refs, nil
}

func findLinearIssue(ctx context.Context, client linear.Clienter, selector string) (linear.Issue, error) {
	issue, found, err := client.FindIssue(ctx, selector)
	if err != nil {
		return linear.Issue{}, linearAPIError("failed to find issue", err)
	}
	if !found {
		return linear.Issue{}, linearIssueNotFound(selector)
	}
	return issue, nil
}

func resolveLinearIssueRef(ctx context.Context, client linear.Clienter, selector string) (linearIssueRefOutput, error) {
	issue, err := findLinearIssue(ctx, client, selector)
	if err != nil {
		return linearIssueRefOutput{}, err
	}
	output, err := newLinearIssueRefOutput(linear.IssueRef{ID: issue.ID, Identifier: issue.Identifier})
	if err != nil {
		return linearIssueRefOutput{}, linearAPIError("invalid issue response", err)
	}
	return output, nil
}

func newLinearIssueRefOutput(issue linear.IssueRef) (linearIssueRefOutput, error) {
	id, ok := linearGraphQLIDString(issue.ID)
	if !ok {
		return linearIssueRefOutput{}, fmt.Errorf("issue ID is not a string: %T", issue.ID)
	}
	return linearIssueRefOutput{ID: id, Identifier: string(issue.Identifier)}, nil
}

func newOptionalLinearIssueRefOutput(issue linear.IssueRef) (*linearIssueRefOutput, error) {
	if issue.ID == nil {
		return nil, nil
	}
	output, err := newLinearIssueRefOutput(issue)
	if err != nil {
		return nil, err
	}
	return &output, nil
}

func newLinearBlockedByOutputs(issue linear.Issue) ([]linearIssueRefOutput, error) {
	relations, err := issue.BlockedBy()
	if err != nil {
		return nil, err
	}
	outputs := make([]linearIssueRefOutput, 0, len(relations))
	for _, relation := range relations {
		output, err := newLinearIssueRefOutput(relation.Issue)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}
