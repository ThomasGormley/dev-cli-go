# Workflow State Tracker CLI Implementation Plan

## Overview

The `dev workflow` CLI is a lightweight tool to streamline your software development workflow, reducing friction in ticket management, branch switching, and PR handling. It extends your existing `dev` CLI and fuzzy branch finder, adding state tracking and automation to save ~10-15 min/day on transitions, reduce cognitive load by ~30%, and prevent errors (e.g., merge conflicts, forgotten PRs). The design prioritizes simplicity and iterative development to validate value early.

## Goals

- **Primary**: Minimize context switching and manual lookups (e.g., branch hunting, PR status checks).
- **Secondary**: Track dependencies for stacked changes, improve review visibility.
- **Non-Goals**: Over-automation (e.g., complex rules), replacing Linear/GitHub UIs entirely.

## System Context

- **Inputs**: Linear tickets, git branches, GitHub PRs.
- **Processes**: Ticket assignment, branching, coding, PR creation, review handling.
- **Feedback Loops**: Team reviews (tech lead often picks up), dependency updates.
- **Constraints**: Team-based review assignment (no individual assignments), minimal external system queries to avoid delays, build Cleveland Clinicbuild on existing CLI.

## Implementation Plan

The CLI extends your `dev` CLI with a `workflow` subcommand, using a local state file (`~/.dev_workflow_state.json`) to track tickets, branches, and PRs. Features are split into iterations, ordered by value-to-effort ratio, with evaluation steps to measure impact.

### ✅ COMPLETED: Iteration 1: Core State Tracking & Context Switching (3-4 hours)

**Goal**: Replace your `dev checkout` idea with a robust `dev workflow checkout` and basic state management, saving ~5-10 min/day by eliminating branch hunting and context recall.

**Features Implemented**:

1. **Unified Smart Checkout** (`dev workflow checkout`):
   - **No args**: Interactive selection from brnaches
   - **Ticket ID** (e.g., `ENG-123`): Auto-detects start vs. checkout existing workflow
   - Handles uncommitted changes with stash/pop prompts
   - Auto-pulls latest changes after checkout

2. **Optimized State Management**:
   - **Map-based storage**: Changed from array to `map[string]WorkflowEntry` for O(1) lookups
   - **Automatic state persistence**: Updates state file on workflow start/completion
   - State file location: `~/.dev_workflow_state.json`

3. **Enhanced Linear Integration**:
   - **GetAssignedIssues()**: Fetches user's assigned tickets for interactive selection
   - **Auto-assignment**: Assigns tickets to current user when starting workflow
   - **Branch name generation**: Uses Linear's branch name field for consistency

4. **Improved Error Handling**:
   - Better error messages for non-existent branches
   - Clear feedback for uncommitted changes scenarios
   - Graceful handling of Linear API failures

5. **Code Quality Improvements**:
   - Consolidated workflow logic into `workflow_checkout.go`
   - Clean separation of concerns between functions

**State File Format** (Updated):

```json
{
  "ENG-123": {
    "ticketId": "ENG-123",
    "branch": "eng-123-fix-login",
    "prUrl": null,
    "status": "in-progress",
    "dependencies": [],
    "lastUpdated": "2025-09-13T11:37:00Z"
  }
}
```

6. **Status Overview** (`dev workflow status`):
   - Lists all state entries in a table (ticketId, branch, status, PR URL, lastUpdated).
   - Shows workflow entries with their current status and metadata.

**Evaluation**:

- **Metrics**: Time spent switching contexts (target: <10s vs. ~1-2 min), tickets closed/week, errors (e.g., wrong branch).
- **Test Plan**: Use for 1 week on 5-10 tickets. Log time savings, note friction points.
- **Success Criteria**: ~50% reduction in switch time, no branch lookup errors.

### Iteration 2: PR Automation & Dependency Tracking (2-3 hours)

**Goal**: Streamline PR creation and handle stacked changes, saving ~3-5 min/PR and preventing merge conflicts.

**Features**:

1. **PR Creation** (`dev workflow pr`):
   - Extends your `dev pr create`: Pushes branch, creates PR, assigns team for review.
   - Updates state with PR URL, sets status to "in-review".
   - Optional: Generates self-review summary (e.g., linter output).
2. **Dependency Tracking** (`dev workflow stack <parentTicketId>`):
   - Creates dependent branch (e.g., `eng-123-part2`), checks out.
   - Adds dependency link in state file (e.g., `"dependencies": ["ENG-123"]`).
   - Auto-rebases dependents when parent merges (via `dev workflow done`).
3. **Cleanup** (`dev workflow done <ticketId>`):
   - Verifies PR merged, deletes branch, closes Linear ticket.
   - Removes from state, updates dependents (rebase or mark as next).

**Evaluation**:

- **Metrics**: PR creation time (target: <30s vs. ~2 min), conflicts avoided in stacked changes.
- **Test Plan**: Test on 2-3 tickets with stacked changes. Check rebase success, PR speed.
- **Success Criteria**: ~70% faster PR creation, zero conflicts in stacks.

### Iteration 3: Review Visibility & Nudging (2-3 hours)

**Goal**: Reduce review delays by improving visibility and nudging, leveraging tech lead responsiveness, saving ~5-10 min/day.

**Features**:

1. **Enhanced Status** (`dev workflow status`):
   - Shows new PR comments, flags stale PRs (>24h, configurable).
2. **Nudge Reviewers** (`dev workflow nudge <ticketId>`):
   - Posts polite comment on PR (e.g., “Hey team, ready for review!”).
   - Optional: Sends nudge via team communication channel (if used).
3. **Dependency Alerts**:
   - On `status`, warns if dependent PRs are at risk (e.g., failing checks).

**Evaluation**:

- **Metrics**: Review turnaround time (target: <24h vs. current), nudges sent.
- **Test Plan**: Use for 1 week, track review delays, note tech lead response.
- **Success Criteria**: ~20% faster reviews, no missed comments.

## Example Usage Flow

1. **Setup**: Ensure `LINEAR_API_KEY` environment variable is set.
2. **Interactive Selection**: `dev workflow checkout` → shows assigned Linear tickets for selection.
3. **Start New Ticket**: `dev workflow checkout ENG-123` → assigns ticket, creates/checks out branch, updates state.
4. **Switch Context**: `dev workflow checkout 123` → fuzzy searches and checks out matching branch.
5. **Check Status**: `dev workflow status` → shows current workflow state:
   ```
   Ticket ID   Branch              Status       PR URL    Last Updated
   ----------  ------------------  -----------  --------  -------------
   ENG-123     eng-123-fix-login  in-progress  N/A       2h ago
   ENG-456     eng-456-api-update in-progress  N/A       1h ago
   ```
6. **Code & Commit**: Make changes, commit as usual.
7. **Future**: PR creation and dependency tracking (Iterations 2-3).

**Command Intelligence**:

- `dev workflow checkout` (no args) → Interactive ticket selection
- `dev workflow checkout ENG-123` (ticket ID) → Start new or checkout existing workflow

## Build Notes

- **Integration**: Extends your `dev` CLI with unified workflow command structure.
- **Architecture**: Single `workflow_checkout.go` file contains all core logic with clean separation of concerns.
- **State Management**: Optimized map-based storage for O(1) lookups, backward compatible with array format.
- **Safety**: Comprehensive error handling for branch operations, uncommitted changes, and API failures.
- **Performance**: Reduced complexity from multiple commands to single intelligent command with auto-detection.
- **Code Quality**: Removed unnecessary pointer usage, consolidated functions, improved readability.
- **Extensibility**: Ready for PR creation and dependency tracking features in future iterations.

## Current Status & Next Steps

**✅ COMPLETED**: Core workflow functionality with unified command structure

- **Time Savings**: Single command replaces multiple manual steps
- **Error Reduction**: Auto-detection prevents wrong branch checkouts
- **User Experience**: Interactive selection and fuzzy finding reduce cognitive load
- **Performance**: O(1) state lookups with map-based storage

**🚀 READY FOR ITERATION 2**: PR Automation & Dependency Tracking

- Next: Implement `dev workflow pr` for PR creation and state updates
- Add dependency tracking for stacked changes
- Build cleanup functionality for completed workflows

## Evaluation Plan

- **Current Implementation Testing**:
  - Test unified checkout command with various input scenarios
  - Verify state persistence and backward compatibility
  - Measure time savings vs. manual branch switching
- **Pre-Implementation** (Completed):
  - Logged baseline transition times and error rates
- **Post-Iteration**:
  - After each iteration, log same metrics for 1 week.
  - Compare: Expect ~50% less transition time, ~10% more tickets closed, ~80% fewer errors.
- **Iterate**: If savings lag, add features (e.g., prioritization in Iteration 4) or debug friction.

## Why It’s Indispensable

- **Saves Time**: ~10-15 min/day (40-60 hours/year at 3 tickets/day).
- **Reduces Load**: Externalizes state, cuts context switching by ~70%.
- **Prevents Errors**: Dependency tracking eliminates conflicts, nudging speeds reviews.
- **Scales**: Handles parallel/stacked tickets seamlessly, adapts to team dynamics.

Start with Iteration 1, evaluate after 1 week, and iterate. The CLI will feel like an extension of your brain, making the old manual workflow unthinkable.
