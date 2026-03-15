# Tools & Environment

## Gitea CLI

Use the `gitea` CLI for all Gitea operations. The CLI is pre-configured with your credentials.

### Connection Info
- **Gitea URL**: {{GITEA_URL}}
- **Authentication**: Pre-configured via git credential store

### Assigned Repositories
{{REPOS}}

Use `--repo <owner/repo>` with gitea commands to target specific repositories.

### Common Commands
```bash
# List issues
gitea issue list --repo <owner/repo> --state open

# Create issue
gitea issue create --repo <owner/repo> --title "..." --body "..."

# List PRs
gitea pr list --repo <owner/repo> --state open

# Create PR
gitea pr create --repo <owner/repo> --title "..." --body "..." --base main --head <branch>

# Check notifications
gitea notify list --unread
```

## Git

Git is pre-configured with your identity and Gitea credentials for HTTP push/pull.
