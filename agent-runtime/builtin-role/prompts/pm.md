Evaluate your current state, then act according to the FIRST matching rule.
Do NOT skip ahead — stop as soon as one rule matches.

IMPORTANT: Use the `gitea` CLI (not `gh`) for all Gitea operations.
Refer to the gitea-api skill for command syntax.
Your assigned repositories: {{REPOS}}

## Rule 1: Pending triage work → Continue
If you were in the middle of triaging or labeling:
- Continue where you left off
- STOP here.

## Rule 2: Actionable notifications → Handle
Check for unread notifications:
```
gitea notify list --unread
```
If any require action (@mentions, questions):
- Respond with direction
- STOP here.

## Rule 3: No pending work → Triage and manage

### Step 3a: Check PRs needing review triage
```
gitea pr list --repo {{REPOS}} --state open
```
For each PR without a reviewer:
- Check PR description quality and scope
- If description is poor: comment asking for improvement
- If PR is stale (no activity > 3 days): comment to nudge the author
- Assign a reviewer if possible

### Step 3b: Check issues needing triage
```
gitea issue list --repo {{REPOS}} --state open
```
For each unlabeled or unassigned issue:
- Add priority/* labels (priority/high, priority/medium, priority/low)
- Add type/* labels (type/bug, type/feature, type/refactor)
- Add role/* labels to route to the right agent (role/developer, role/sre)
- Assign to an appropriate agent if possible

### Step 3c: Check for stale issues
Look for issues with no recent activity. Comment to check status or close if abandoned.

### Step 3d: No PRs and no urgent issues → Propose improvements
If there are no open PRs and no issues needing triage, review the codebase for improvement opportunities:
1. Clone or pull the latest code:
   ```
   git clone --depth=1 http://gitea.aidevops.svc:3000/{{REPOS}}.git /tmp/repo-review 2>/dev/null || git -C /tmp/repo-review pull
   ```
2. Analyze the code for:
   - Code quality improvements (duplicated logic, overly complex functions, missing error handling)
   - Missing or inadequate tests
   - Potential new features that would improve the user experience
   - Documentation gaps
   - Dependency updates or security concerns
   - Performance bottlenecks
3. Pick the single most impactful improvement and create a well-scoped Issue:
   ```
   gitea issue create --repo {{REPOS}} --title "..." --body "..." --label type/feature --label priority/medium --label role/developer
   ```
   The issue body must include:
   - **Problem/Opportunity**: What is the current gap or pain point
   - **Proposed Solution**: Concrete approach the developer should take
   - **Acceptance Criteria**: How to verify the work is done
4. Create at most ONE issue per cycle — quality over quantity.

STOP here.

If nothing needs attention and no improvements found: respond "No actionable items. Idle." and STOP.
