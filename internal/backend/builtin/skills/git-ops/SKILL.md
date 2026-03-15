# Git Operations Skill

## Clone Strategy

Use shallow clone through Gitea directly (credentials are pre-configured via `.netrc`):

```bash
# Clone via Gitea (credentials handled automatically)
git clone --depth=1 http://gitea.aidevops.svc:3000/{org}/{repo}.git
```

## Git Configuration

Configure git user identity using the agent's identity:

```bash
git config user.email "${AGENT_ID}@platform.local"
git config user.name "${AGENT_ROLE} Agent"
```

## Branch Naming Convention

Format: `feat/issue-{id}-{slug}`

Examples:
- `feat/issue-42-fix-login-bug`
- `fix/issue-15-memory-leak`
- `docs/issue-8-update-readme`
- `refactor/issue-23-extract-utils`

```bash
git checkout -b feat/issue-42-fix-login-bug
```

## Commit Message Format

```
type(scope): description

Closes #42
```

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`

Examples:
```bash
git commit -m "feat(auth): implement JWT refresh token

Closes #42"

git commit -m "fix(api): handle null pointer in user lookup

Closes #15"

git commit -m "docs(readme): add deployment instructions

Closes #8"
```

## Push and PR Creation Workflow

Complete workflow from branch creation to PR:

```bash
# 1. Configure identity
git config user.email "${AGENT_ID}@platform.local"
git config user.name "${AGENT_ROLE} Agent"

# 2. Create feature branch
git checkout -b feat/issue-42-fix-login-bug

# 3. Make changes and commit
git add -A
git commit -m "feat(auth): implement JWT refresh token

Closes #42"

# 4. Push the branch
git push origin feat/issue-42-fix-login-bug

# 5. Create pull request
gitea pr create --repo platform/platform-demo \
  --title "feat(auth): implement JWT refresh token" \
  --body "## Changes
- Add refresh token endpoint
- Add token rotation logic
- Add integration tests

Closes #42" \
  --head feat/issue-42-fix-login-bug \
  --base main
```

## Common Operations

### Check current branch

```bash
git branch --show-current
```

### View recent commits

```bash
git log --oneline -10
```

### View diff before committing

```bash
git diff --stat
git diff
```

### Stash and restore changes

```bash
git stash
git stash pop
```

### Rebase on latest main

```bash
git fetch origin main
git rebase origin/main
```
