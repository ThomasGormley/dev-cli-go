# AGENTS.md

This document provides guidelines for agentic coding agents working in this repository.

## Build, Lint, and Test Commands

### Build
```bash
make build
# or
go build -o ./bin/dev
```

### Run All Tests
```bash
make test
# or
go test ./...
```

### Run a Single Test
```bash
go test ./internal -run TestPrTitleFromBranch
go test ./internal/... -run "TestList"
```

### Watch Tests (requires twatchy or similar)
```bash
go test ./... -v | watch -t
```

### Clean Build Artifacts
```bash
make clean
```

### Install Binary
```bash
make install
```

## Code Style Guidelines

### Imports
- Use standard Go import grouping:stdlib first, then third-party, then internal
- Use aliased imports for internal packages (`cli "github.com/thomasgormley/dev-cli-go/internal"`)
- Group imports with clear separation:

```go
import (
    "bytes"
    "context"
    "fmt"
    "io"

    "github.com/thomasgormley/dev-cli-go/internal/gh"
    "github.com/urfave/cli/v2"
)
```

### Formatting
- Run `gofmt` before committing (project uses standard Go formatting)
- Keep lines under 120 characters where reasonable
- Use vertical spacing between related functions (1-2 blank lines)
- No comments on closing braces unless clarifying control flow

### Naming Conventions
- **Interfaces**: Use "er" suffix (`GitHubClienter`, `Reader`, `Writer`)
- **Variables**: camelCase for local vars, PascalCase for exported
- **Constants**: PascalCase for exported, camelCase for unexported; use grouped const blocks for related values
- **Functions**: Verb-based for actions (`CreatePR`, `ListPRs`, `CurrentBranch`)
- **Packages**: lowercase, single word or short phrase (`gh`, `git`, `diary`, `clipboard`)
- **Receiver variables**: 1-2 letters matching type (`g *ghClient`, `r *repo`)

### Error Handling
- Return errors early; avoid `err != nil` blocks mid-function
- Wrap errors with context using `fmt.Errorf("action: %w", err)`
- Use `errors.New` for sentinel errors without context
- CLI errors should use `cli.Exit(err, 1)` for urfave/cli actions
- Check errors at call site, don't ignore with `_`

### Types and Structs
- Use struct embedding sparingly; prefer explicit field names
- JSON structs use camelCase field names with `json:"fieldName"` tags
- Group related constants in const blocks with type specified once
- Embed `context.Context` as first parameter in public functions

### Testing
- Table-driven tests with `[]struct{ name, input, expected }`
- Use `t.Run` subtests for clarity: `for _, tt := range tests { t.Run(tt.name, ...)`
- Test files named `*_test.go` alongside implementation
- Group test assertions; use `t.Errorf` with descriptive messages
- Avoid magic numbers; use named constants in tests

### Project Structure
- `main.go` at root; all logic in `internal/` packages
- Internal packages organized by domain: `gh/`, `git/`, `diary/`, `clipboard/`, `print/`, `linear/`, `spinner/`, `editor/`
- Public types in each package; unexported types prefixed with lowercase
- CLI commands defined in `internal/run.go` using urfave/cli/v2

### CLI Framework
- Commands defined with `&cli.Command` structs in `run.go`
- Use `cli.ActionFunc` for action handlers
- Pass `io.Writer` (stdout/stderr) for testability
- Return `error` from actions; use `cli.Exit` for failures
- Flags defined with `&cli.StringFlag`, `&cli.BoolFlag` with `Aliases` for short forms

### Git Integration
- Heavy use of `exec.Command` for git operations (see `internal/git/git.go`)
- Use `exec.CommandContext` with context for cancellation
- Parse output with `bytes.TrimSpace` and `bytes.Split`
- Handle exit codes explicitly in git operations

### Pull Request Workflow
- PR functions in `internal/gh/gh.go` wrap `gh` CLI
- JSON fields defined in var `jsonFields` slice for `gh pr status --json`
- PR structs mirror GitHub API responses with json tags
- Merge state constants defined in grouped const block (CLEAN, DRAFT, BLOCKED, etc.)

## Key Dependencies
- `github.com/urfave/cli/v2` - CLI framework
- `github.com/google/go-github/v69` - GitHub API client
- `github.com/AlecAivazis/survey/v2` - Interactive prompts
- `github.com/sst/opencode-sdk-go` - OpenCode agent integration
- `github.com/stretchr/testify` - Testing assertions

## General Principles
- Keep functions under 50 lines when possible
- Prefer early returns over nested conditionals
- Use meaningful variable names; avoid single-letter vars except receivers
- Handle context cancellation in long-running operations
- Return zero values for errors (e.g., `PRStatusResponse{}`) alongside error
- Log with `log.Printf` for debugging; use `fmt.Fprintln` for user output

## HTTP Handler Patterns

### Package Naming
- Avoid stdlib names like `http`; use single lowercase words (`serve`, `api`, `web`)
- Follow existing package conventions in `internal/` (`gh`, `git`, `diary`, etc.)

### Configuration Structs
- Use `HandleOpts` or `HandlerConfig` structs for multiple handler options
- Separates CLI concerns (binding address) from handler concerns (CORS origins)
- Example:
```go
type HandleOpts struct {
    GitHubUser     string
    GitHubClient   *githubapi.Client
    AllowedOrigins []string
}
```

### CLI vs Handler Separation
- CLI actions (e.g., `handleServe`) stay in `cli` package
- HTTP handlers and logic go in dedicated packages (`internal/serve`)
- Handler package exports `Handle(opts)` function; CLI creates and configures

### Type Exports
- Export types needed for JSON marshal/unmarshal or cross-package signatures
- Keep internal request/response structs unexported

## GitHub PR Bot

### GitHub GraphQL Structure
- **Top-level comments**: `pullRequest.comments.nodes` (no path/line/diffHunk)
- **Inline comments**: `pullRequest.reviews.nodes[].comments.nodes` (has path, line, diffHunk)
- Distinguish by checking `c.FilePath != ""`

### Comment Operations
- PR review comments → `CreatePullRequestCommentReaction`
- Issue comments → `CreateReaction`
- Reply to inline → `CreateReviewCommentReply`

### Agent Repo Caching
- Agent caches repos at `~/.devagent/repos/{owner}/{repo}/`
- Resets to `origin/{branch}` on each run to sync state
- User's local dev directory is separate from agent's cached repo
