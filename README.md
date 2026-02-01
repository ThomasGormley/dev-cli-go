# dev-cli 🛠

Development toolbox with common utilities and scripts

## Foo Bar Baz

fake pull request

## `dev pr` Commands

### `dev pr create`

Create a new pull request.

```bash
dev pr create [OPTIONS]
```

**Options:**

- `-t, --title <string>` - Title of the pull request
- `-b, --body <string>` - Body of the pull request
- `-B, --base <string>` - Base branch (defaults to repository default, can use `TEAM_BRANCH` env var)
- `-d, --draft` - Mark the pull request as a draft (default: true)

**Usage:**

Create a PR with a title and body:
```bash
dev pr create -t "Fix login bug" -b "This PR fixes the login issue"
```

Create a draft PR:
```bash
dev pr create -t "Work in progress" --draft
```

If title or body are not provided, you will be prompted to enter them. If a PR template exists in the repository, it will be used for the body when not specified.

After creating the PR, you will be asked if you want to open it in your browser.
