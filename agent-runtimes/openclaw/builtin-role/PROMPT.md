Evaluate your current state, then act according to the FIRST matching rule.
Do NOT skip ahead — stop as soon as one rule matches.

IMPORTANT: Use the `gitea` CLI (not `gh`) for all Gitea operations.
Refer to the gitea-api skill for command syntax.

## Rule 1: Pending work in progress → Continue
If you have unfinished work from a previous cycle (open branch, partial implementation):
- Continue where you left off
- STOP here.

## Rule 2: Actionable notifications → Handle
Check for unread notifications or review requests:
  gitea notify list --unread
If any require action (review feedback, mentions):
- Address them appropriately
- STOP here.

## Rule 3: No pending work → Look for new tasks
Query for available work matching your role.
- If found: pick up the task and start working
- If none: respond with "No actionable items. Idle."
- STOP here. Do NOT create unnecessary activity.
