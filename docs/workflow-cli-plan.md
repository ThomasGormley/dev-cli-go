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

### Iteration 1: Core State Tracking & Context Switching (3-4 hours)

**Goal**: Replace your `dev checkout` idea with a robust `dev workflow switch` and basic state management, saving ~5-10 min/day by eliminating branch hunting and context recall.

**Features**:

1. [SKIP] **State File Setup** (`dev workflow init`):
   - Creates a state file to track active tickets, branches, PRs, and statuses:
     ```json
     [
       {
         "ticketId": "ENG-123",
         "branch": "eng-123-fix-login",
         "prUrl": null,
         "status": "in-progress", // "in-progress", "in-review", "merged"
         "dependencies": [],
         "lastUpdated": "2025-09-13T11:37:00Z"
       }
     ]
     ```
   - Prompts for configuration (e.g., external system credentials, repo path).
2. **Start Ticket** (`dev workflow start <ticketId>`):
   - Assigns ticket to you in Linear.
   - Generates branch name (e.g., `eng-123-<slug>`), copies to clipboard, checks out branch.
   - Adds entry to state file with "in-progress" status.
3. **Switch Context** (`dev workflow switch <fuzzyQuery>`):
   - Fuzzy searches state and branches (integrates with your fuzzy finder).
   - Checks out matching branch, pulls latest.
   - Displays ticket details and PR status if applicable.
4. **Status Overview** (`dev workflow status`):
   - Lists all state entries in a table (ticketId, branch, status, PR URL, lastUpdated).
   - Checks external systems for PR updates only when needed.

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

1. **Setup**: `dev workflow init` (configure credentials, repo).
2. **Start Ticket**: `dev workflow start ENG-123` → assigns, creates/checks out `eng-123-fix-login`.
3. **Code & PR**: Code, then `dev workflow pr` → creates PR, assigns team.
4. **Start Stacked Ticket**: `dev workflow stack ENG-123` → creates `eng-123-part2`.
5. **Check Status**: `dev workflow status` → shows:
   ```
   | Ticket   | Branch              | Status     | PR URL | Last Updated |
   |----------|---------------------|------------|--------|--------------|
   | ENG-123  | eng-123-fix-login  | in-review  | [link] | 2h ago       |
   | ENG-123  | eng-123-part2      | in-progress| N/A    | now          |
   Warning: ENG-123 PR stale (2h).
   ```
6. **Nudge**: `dev workflow nudge ENG-123` → pings team.
7. **Switch**: `dev workflow switch 123` → checks out `eng-123-fix-login`, shows comments.
8. **Finish**: `dev workflow done ENG-123` → deletes branch, closes ticket, rebases `eng-123-part2`.

## Build Notes

- **Integration**: Extends your `dev` CLI and fuzzy finder.
- **Safety**: Validate inputs (e.g., ticket exists), handle external system failures (fallback to manual), backup state file.
- **Extensibility**: Support hooks (e.g., post-start runs linter) via config.

## Evaluation Plan

- **Pre-Implementation**:
  - Log time spent on transitions (start, switch, PR) for 1 week.
  - Count tickets closed, errors (e.g., wrong branch, conflicts).
  - Rate focus (1-10) daily.
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
