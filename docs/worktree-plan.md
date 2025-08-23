# Worktree Feature Implementation Plan

## Overview
<<<<<<< HEAD

This document outlines the plan for implementing worktree integration in the `dev` CLI tool to enhance the development workflow with git worktrees and tmux sessions.

## Current Workflow

=======
This document outlines the plan for implementing worktree integration in the `dev` CLI tool to enhance the development workflow with git worktrees and tmux sessions.

## Current Workflow
>>>>>>> aa4da08 (Implement basic branch checkout with worktree creation)
- Using `tmux-sessionizer` to manage development environments
- Existing git aliases for worktree management (`gwtls`, `gwta`, etc.)
- Need to streamline branch checkout with automatic worktree creation

## Proposed Workflow
<<<<<<< HEAD

=======
>>>>>>> aa4da08 (Implement basic branch checkout with worktree creation)
1. `dev checkout` - Shows in-progress linear tickets/branches or creates one
2. Automatically checks out branch to a worktree in the same parent folder as base repo
3. If checking out a branch that exists as a tmux session, open that session

## Phased Implementation

### Phase 1: Basic Branch Checkout with Worktree (MVP)

#### Goals
<<<<<<< HEAD

=======
>>>>>>> aa4da08 (Implement basic branch checkout with worktree creation)
- Implement `dev checkout <branch>` command
- Automatically create worktree if it doesn't exist
- Use simple directory structure in parent folder

#### Implementation Steps

1. **Add git worktree functions to `internal/git/git.go`**
   - `ListWorktrees()` - List all worktrees for the repo
   - `CreateWorktree(branch, path string)` - Create a new worktree
   - `GetWorktreePathForBranch(branch string)` - Get expected worktree path for branch

2. **Implement basic checkout logic**
   - Check if worktree exists for branch
   - If not, create it in parent directory
   - If yes, switch to that directory

3. **Test the basic workflow**
   - `dev checkout feature-branch` creates worktree
   - Verify worktree location works with tmux-sessionizer

#### Status
<<<<<<< HEAD

=======
>>>>>>> aa4da08 (Implement basic branch checkout with worktree creation)
✅ Implemented `dev checkout <branch>` command
✅ Automatically creates worktree if it doesn't exist
✅ Uses simple directory structure in parent folder
✅ Basic workflow tested successfully
<<<<<<< HEAD
<<<<<<< HEAD
✅ Added branch prompting when no branch is provided
✅ Fuzzy find over list of local branches from most recent descending
=======
>>>>>>> aa4da08 (Implement basic branch checkout with worktree creation)
=======
✅ Added branch prompting when no branch is provided
✅ Fuzzy find over list of local branches from most recent descending
>>>>>>> 1f5b279 (Add branch prompting and fuzzy find for checkout command)

### Phase 2: Tmux Session Integration

#### Goals
<<<<<<< HEAD

=======
>>>>>>> aa4da08 (Implement basic branch checkout with worktree creation)
- Integrate with tmux sessions for seamless workflow

#### Implementation Steps

1. **Add tmux utility functions**
   - `HasSession(name string)` - Check if tmux session exists
   - `CreateSession(name, path string)` - Create new tmux session
   - `AttachSession(name string)` - Attach to existing session

2. **Enhance checkout command**
   - Check for existing tmux session when checking out branch
   - Attach to session if it exists
   - Create new session and worktree if needed

<<<<<<< HEAD
#### Status

✅ Added tmux session creation for worktrees
✅ Check for existing tmux sessions
✅ Inform user how to attach to session

### Phase 3: Branch Selection Interface

#### Goals

=======
### Phase 3: Branch Selection Interface

#### Goals
>>>>>>> aa4da08 (Implement basic branch checkout with worktree creation)
- Implement interactive branch selection
- Show in-progress tickets/branches

#### Implementation Steps

1. **Add branch listing functionality**
   - Show remote branches for linear tickets
   - Show local branches with worktree status
   - Implement fuzzy finding interface

2. **Integrate with checkout command**
   - `dev checkout` without arguments shows branch selector
   - Allow creating new branches from selector

### Phase 4: Cleanup and Maintenance

#### Goals
<<<<<<< HEAD

=======
>>>>>>> aa4da08 (Implement basic branch checkout with worktree creation)
- Implement worktree cleanup functionality

#### Implementation Steps

1. **Add cleanup commands**
   - `dev worktree cleanup` - Identify merged worktrees
   - Automatic cleanup options

## Directory Structure
<<<<<<< HEAD

=======
>>>>>>> aa4da08 (Implement basic branch checkout with worktree creation)
```
parent/
├── repo/              # Main repository
├── repo-branch1/      # Worktree for branch1
├── repo-branch2/      # Worktree for branch2
└── repo-feature-name/ # Worktree for feature branches
```

## Benefits
<<<<<<< HEAD

- Seamless workflow between git worktrees and tmux sessions
- Reduced context switching
- Cleaner organization of feature work
- Integration with existing tmux-sessionizer setup
=======
- Seamless workflow between git worktrees and tmux sessions
- Reduced context switching
- Cleaner organization of feature work
- Integration with existing tmux-sessionizer setup
>>>>>>> aa4da08 (Implement basic branch checkout with worktree creation)
