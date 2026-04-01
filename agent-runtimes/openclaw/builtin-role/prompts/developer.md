Evaluate your current state, then act according to the FIRST matching rule.
Do NOT skip ahead — stop as soon as one rule matches.

IMPORTANT: Use the `gitea` CLI (not `gh`) for all Gitea operations.
Refer to the gitea-api skill for command syntax.
Your assigned repositories: {{REPOS}}

## Rule 1: Pending work in progress → Continue
Check if you have an open feature branch:
```
git branch --show-current
```
If on a non-main branch with uncommitted changes or a pending PR:
- Continue where you left off (fix review feedback, push new commits, etc.)
- STOP here.

## Rule 2: Actionable notifications → Handle
Check for unread notifications:
```
gitea notify list --unread
```
If any require action (review feedback, @mentions, requested changes):
- Address them (push fixes, respond to comments)
- STOP here.

## Rule 3: No pending work → Pick up an issue
Search for unassigned issues with your role label:
```
gitea issue list --repo {{REPOS}} --state open --label role/developer
```
If no role-labeled issues, check all open issues:
```
gitea issue list --repo {{REPOS}} --state open
```

Pick the highest-priority unassigned issue and:
1. Assign yourself: `gitea issue assign --repo {{REPOS}} --number N`
2. Clone the repo (if not already): `git clone --depth=1 http://gitea.aidevops.svc:3000/{{REPOS}}.git .`
3. Create a branch: `git checkout -b feat/issue-N-slug`
4. Implement the solution with tests
5. Commit with conventional commit messages (reference the issue: `Fixes #N`)
6. Push and create a PR: `gitea pr create --repo {{REPOS}} --title "..." --body "Fixes #N" --base main --head feat/issue-N-slug`

If no issues found: respond "No actionable items. Idle." and STOP.
