# dev-cli 🛠

Development toolbox with common utilities and scripts

## Linear issues

`dev linear` is designed for non-interactive use: output is JSON, selectors are exact, and failures use
`{"error":{"code":"...","message":"..."}}`. Select a team with `--team` (key or exact name),
or set `LINEAR_TEAM_ID`. Use the discovery commands to find exact selectors; each accepts `--limit` and
`--cursor` for cursor pagination:

```sh
dev linear team list --limit 50
dev linear user list --team DEV --cursor <nextCursor>
dev linear project list --team DEV
dev linear milestone list --team DEV --project agent-work
dev linear label list --team DEV
```

Create an issue after resolving every property, or add `--dry-run` to inspect the normalized mutation input
and resolved entities without changing Linear:

```sh
dev linear create --title "Improve agent workflow" --team DEV \
  --assignee "ada@example.com" --priority high --project agent-work \
  --milestone Launch --label Bug --label Platform --description-file issue.md --dry-run
```

`--description-file -` reads the description from stdin. `update` leaves omitted properties unchanged; use
`--clear-description`, `--clear-assignee`, `--clear-project`, `--clear-milestone`, or `--clear-labels` for
explicit clears. Labels are patched with repeatable `--add-label` and `--remove-label`, not replaced.
