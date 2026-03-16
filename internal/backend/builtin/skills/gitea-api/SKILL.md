# Gitea API Skill

## Overview

The `gitea` binary is pre-installed at `/usr/local/bin/gitea`. No token or URL configuration is needed -- it reads credentials from `/agent/secrets/gitea-token` and uses the platform gateway automatically.

All commands follow the pattern: `gitea <resource> <action> [flags]`

## Issue Commands

### List Issues

```bash
# List open issues (default limit 20)
gitea issue list --repo platform/platform-demo --state open --limit 20

# Filter by labels
gitea issue list --repo platform/platform-demo --label priority/high,type/bug

# List closed issues
gitea issue list --repo platform/platform-demo --state closed
```

### Create an Issue

```bash
gitea issue create --repo platform/platform-demo \
  --title "Fix login bug" \
  --body "Description here" \
  --labels type/bug,priority/high
```

### Assign an Issue

```bash
# Assign to yourself
gitea issue assign --repo platform/platform-demo --number 42

# Assign to a specific user
gitea issue assign --repo platform/platform-demo --number 42 --assignee alice
```

### Comment on an Issue

```bash
gitea issue comment --repo platform/platform-demo --number 42 \
  --body "Working on this now"
```

### Close an Issue

```bash
gitea issue close --repo platform/platform-demo --number 42
```

## Pull Request Commands

### List Pull Requests

```bash
gitea pr list --repo platform/platform-demo --state open
```

### Create a Pull Request

```bash
gitea pr create --repo platform/platform-demo \
  --title "feat: add login" \
  --body "PR description" \
  --head feat/issue-42-login \
  --base main
```

### Review a Pull Request

```bash
# Approve
gitea pr review --repo platform/platform-demo --number 5 \
  --event APPROVE --body "LGTM"

# Request changes
gitea pr review --repo platform/platform-demo --number 5 \
  --event REQUEST_CHANGES --body "Please fix tests"
```

### Check PR Review Status

```bash
# List all reviews for PR #5 — shows reviewer, state, and merge-readiness
gitea pr reviews --repo platform/microservices-demo --number 5

# JSON output (for scripting)
gitea --json pr reviews --repo platform/microservices-demo --number 5
```

Text output example:
```
APPROVED     by agent-reviewer
COMMENT      by agent-pm

approved=1 changes_requested=0
status=ready_to_merge
```

A PR is **ready to merge** when the output shows `status=ready_to_merge`.

### Merge a Pull Request

```bash
gitea pr merge --repo platform/platform-demo --number 5 --method squash
```

## Notification Commands

```bash
# List unread notifications
gitea notify list --unread

# List notifications since a timestamp
gitea notify list --since 2024-01-01T00:00:00Z

# Mark a specific notification as read
gitea notify read --thread-id 123

# Mark all notifications as read
gitea notify read-all
```

## File Commands

```bash
# Get file contents from a branch
gitea file get --repo platform/platform-demo --path src/main.go --ref main

# Preview first 20 lines
gitea file get --repo platform/platform-demo --path README.md | head -20
```

## JSON Output

Add the `--json` flag before the resource command for machine-readable JSON output:

```bash
gitea --json issue list --repo platform/platform-demo | jq '.[].title'
gitea --json pr list --repo platform/platform-demo | jq '.[].number'
```

## Standard Labels Reference

| Label              | Color     | Description                        |
|--------------------|-----------|------------------------------------|
| priority/critical  | `#FF0000` | Critical, needs immediate attention|
| priority/high      | `#FF6600` | High priority                      |
| priority/medium    | `#FFCC00` | Medium priority                    |
| priority/low       | `#00CC00` | Low priority                       |
| type/bug           | `#EE0701` | Bug report                         |
| type/feature       | `#84B6EB` | New feature request                |
| type/docs          | `#0075CA` | Documentation                      |
| type/refactor      | `#E4E669` | Code refactoring                   |
| type/test          | `#F9D0C4` | Test related                       |
| role/developer     | `#7057FF` | For developer agents               |
| role/reviewer      | `#008672` | For reviewer agents                |
| role/qa            | `#E4E669` | For QA agents                      |
| role/sre           | `#0052CC` | For SRE agents                     |
