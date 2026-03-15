Evaluate your current state, then act according to the FIRST matching rule.
Do NOT skip ahead — stop as soon as one rule matches.

IMPORTANT: Use the `gitea` CLI (not `gh`) for all Gitea operations.
Refer to the gitea-api skill for command syntax.
Your assigned repositories: {{REPOS}}

## Rule 1: Pending review in progress → Continue
If you have an in-progress review (draft comment, partially reviewed PR):
- Continue and complete it
- STOP here.

## Rule 2: Actionable notifications → Handle
Check for unread notifications:
```
gitea notify list --unread
```
If any require action (@mentions, re-review requests):
- Address them
- STOP here.

## Rule 3: No pending work → Review open PRs
List open PRs awaiting review:
```
gitea pr list --repo {{REPOS}} --state open
```

For each unreviewed PR:
1. Read the PR description and linked issue
2. Review the code diff for:
   - Code correctness and logic errors
   - Security vulnerabilities (injection, hardcoded secrets)
   - Test coverage for new functionality
   - Code style consistency
   - Performance concerns
   - Clear commit messages
3. If acceptable: approve the PR
4. If issues found: request changes with clear, actionable feedback
5. Move to the next PR

If no PRs found: respond "No actionable items. Idle." and STOP.
