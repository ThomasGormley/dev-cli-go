# Standardized Workflow CLI Interface Plan

## Overview
This plan standardizes the UX for `dev workflow` commands (checkout, complete, status) to create a consistent, predictable interface. Core principle: **No args = interactive fuzzy selection** (using `survey.Select` with PageSize=16 for usability). Fuzzy options prioritize context-relevance (e.g., workflows over raw branches) to minimize cognitive load. Args/flags override for direct actions. Builds on existing implementation: map-based state (`~/.dev_workflow_state.json`), Linear integration, safe Git ops. Non-goals: Overhaul state schema; add new commands (focus on existing: checkout, complete, status). Estimated effort: 4-6 hours (refactor prompts/options builders).

## Goals
- **Primary**: Unified patterns reduce scatter (e.g., current-branch inference, ticket args) → One "smart" interactive mode per command, with clear arg fallbacks.
- **Secondary**: Context-aware fuzzy: Tailor options to command intent (e.g., workflows for checkout, not raw branches unless no state). Improve discoverability (e.g., show ticket details in options).
- **UX Principles**:
  - Interactive-first (no args): Fuzzy from most relevant sources (workflows > Linear tickets > branches).
  - Direct args: Ticket ID (e.g., `ENG-123`) for targeted actions; flags for overrides (e.g., `--new`, `--branch`).
  - Safety/Feedback: Always prompt for destructive actions (e.g., stash, delete); concise post-action summaries.
  - Error Handling: Graceful (e.g., "No matching workflows; try a ticket ID?"); warn on missing `LINEAR_API_KEY`.
- **Success Metrics**: User tests show <5s avg interaction time; 0 confusion on "what fuzzy shows" (post-refactor eval: 1-week usage log).

## Standardized Patterns by Command

### 1. `dev workflow checkout` (Context Switching/Start)
   - **Purpose**: Switch to or start a workflow (branch + ticket). Handles new/existing via state/Linear.
   - **No Args (Fuzzy)**: Interactive selection from **active workflows first** (state entries: "🎫 ENG-123: branch (in-progress)"), then **assigned Linear tickets** (if no state: "📋 ENG-456: Description snippet"), fallback to **all local branches** ("🌿 main/feature-branch"). Load state → Fetch user's assigned issues (Linear `GetAssignedIssues`) → Build options/map → Select → If workflow/ticket: Safe checkout (stash prompt if uncommitted) + pull + display details (ticket/branch/status/PR). If raw branch: Safe switch + pull.
     - Why this order? Workflows (state) for quick recall; Linear for new starts; branches for edge (non-ticket work). Limits scatter by nesting contexts.
   - **Ticket ID Arg** (e.g., `checkout ENG-123`): Direct: Check state → If existing: Safe checkout + pull + display. If new: Linear assign/fetch → Prompt branch name (default: issue.BranchName) → Safe checkout + store state ("in-progress").
   - **Flags**:
     - `--new`: Force new workflow (even if state exists; prompt branch).
     - `--branch <name>`: Direct branch switch (bypass fuzzy/ticket; safe checkout).
   - **Flow Safety**: Always: Uncommitted check → Stash/pop prompts. Post: "Switched to [branch] (Ticket: [ID], Status: [S])".
   - **Changes Needed**: Update `buildWorkflowOptions` to include Linear tickets + branches; add flag parsing; merge new/existing logic under arg detection (`looksLikeTicketID`).

### 2. `dev workflow complete` (Cleanup)
   - **Purpose**: Finish workflow (delete branch, update Linear to "Done", remove state). Assumes PR merged (add TODO check via GitHub API later).
   - **No Args (Fuzzy)**: Interactive from **active workflows only** (state entries, prioritize current branch: "🎫 ENG-123: branch [current] (in-progress)"), exclude completed/inactive. No Linear tickets (focus on finishing existing); no raw branches (only state-tracked to avoid accidents).
     - Why? Context is closure: State ensures tied to tickets; current-branch first for in-flow use.
   - **Ticket ID Arg** (e.g., `complete ENG-123`): Direct: Find state by ticket → If found: Confirm delete (prompt if unpushed) + switch to main/pull if current + delete local branch + Linear state update (prompt if not "Done") + delete state.
   - **Flags**:
     - `--force`: Skip prompts (e.g., auto-delete unpushed; use cautiously).
     - `--branch <name>`: Target specific branch (find state by branch).
   - **Flow Safety**: Unpushed check → Force-delete prompt; main branch guard; Linear "Done" prompt. Post: "Completed workflow for [ticket/branch]".
   - **Changes Needed**: Refactor `buildCompleteOptions` to workflows-only (no current fallback if no state); add arg/flag handling; integrate Linear update always (with prompt).

### 3. `dev workflow status` (Overview)
   - **Purpose**: View all workflows (no mutations; read-only).
   - **No Args (Fuzzy)**: N/A (always non-interactive table from **all state entries** sorted by LastUpdated desc). Columns: Ticket ID | Branch | Status | PR URL | Last Updated | Dependencies (if added later). If empty: "No workflows. Start one with `checkout`?".
     - Why no fuzzy? Listing is overview; interactivity would dilute (consider filter flag if expanded).
   - **Ticket ID Arg** (e.g., `status ENG-123`): Show details for single workflow (enhanced: include Linear issue summary if fetched).
   - **Flags**:
     - `--all`: Include inactive/completed (if state tracks them).
     - `--json`: Machine-readable output.
   - **Flow Safety**: None (read-only). Post: Table only.
   - **Changes Needed**: Add arg for single-view; optional Linear fetch for details; sort output.

## Global Patterns & Consistency
- **Fuzzy Option Building**: Shared helper `buildFuzzyOptions(ctx Command, sources []Source)`: Sources = [Workflows (state), LinearTickets (assigned), Branches (git)]. Command-specific filtering (e.g., complete: workflows only).
- **Detection/Args**: Retain `looksLikeTicketID`; add `--help` examples (e.g., "checkout [TICKET] | --branch <name>").
- **Prompts**: Uniform `survey` usage: Confirms for safety; Selects for choices. Add progress indicators if async (e.g., Linear fetch).
- **State Updates**: Auto on start/complete; no changes for status.
- **Edge Cases**: No state/Linear key: Fallback to branches; offline: Warn + proceed with local.
- **Extensibility**: For future (Iteration 2+): Add `pr` (fuzzy from workflows), `stack` (arg: parent ticket → fuzzy child options).

## Implementation Steps
1. **Refactor Helpers** (1-2h): Update `buildWorkflowOptions`/`buildCompleteOptions` for multi-source fuzzy; add flag parsing (urfave/cli).
2. **Per-Command Updates** (2h): Integrate fuzzy/args/flags; test flows (new/existing, empty state).
3. **Testing** (1h): Unit (mock Linear/Git); e2e (manual: no-arg fuzzy, arg direct). Cover: Stash scenarios, unpushed deletes, Linear fails.
4. **Eval** (Ongoing): 1-week use: Log interactions (fuzzy vs arg usage); survey: "Is fuzzy intuitive?" Target: 80% prefer no-arg mode.

## Risks & Mitigations
- Over-prompting: Limit to 1-2 per flow; default safe choices.
- Performance: Cache Linear fetches (e.g., 5min); parallel Git/Linear calls.
- Backward Compat: Preserve old state format; no breaking arg behavior.

This standardizes to intuitive, context-aware UX: Fuzzy adapts to "what you likely want now," args for precision. Next: Implement Iteration 2 PR features with same patterns.