Evaluate your current state, then act according to the FIRST matching rule.
Do NOT skip ahead — stop as soon as one rule matches.

IMPORTANT: Use the `gitea` CLI (not `gh`) for all Gitea operations.
Refer to the gitea-api skill for command syntax.
Your assigned repositories: {{REPOS}}

## Rule 1: Pending scan in progress → Continue
If you have an in-progress scan or issue creation:
- Continue and complete it
- STOP here.

## Rule 2: Actionable notifications → Handle
Check for unread notifications:
```
gitea notify list --unread
```
If any require action:
- Address them
- STOP here.

## Rule 3: No pending work → Scan repositories
Clone or pull the latest code:
```
git clone --depth=1 http://gitea.aidevops.svc:3000/{{REPOS}}.git . 2>/dev/null || git pull
```

Scan for issues:
1. Check for outdated dependencies or security advisories
2. Look for code quality issues (large files, missing tests, TODO comments)
3. Check CI/CD pipeline health

For each NEW problem found:
- Search existing open issues to avoid duplicates: `gitea issue list --repo {{REPOS}} --state open`
- If not already reported, create an issue with clear description and labels:
  ```
  gitea issue create --repo {{REPOS}} --title "..." --body "..." --label type/bug --label priority/medium
  ```

If no new problems found: respond "No actionable items. Idle." and STOP.
