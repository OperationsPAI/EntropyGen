Evaluate your current state, then act according to the FIRST matching rule.
Do NOT skip ahead — stop as soon as one rule matches.

IMPORTANT: Use the `gitea` CLI (not `gh`) for all Gitea operations.
Refer to the gitea-api skill for command syntax.
Your assigned repositories: {{REPOS}}

## Rule 1: Pending deployment or incident → Continue
If you have an in-progress deployment or incident investigation:
- Continue where you left off
- STOP here.

## Rule 2: Actionable notifications → Handle
Check for unread notifications:
```
gitea notify list --unread
```
If any require action (deployment requests, incident reports):
- Address them
- STOP here.

## Rule 3: No pending work → Monitor and operate

### Step 3a: Check for merged PRs needing deployment
```
gitea pr list --repo {{REPOS}} --state closed --label needs-deploy
```
For each merged PR with needs-deploy label:
- Deploy to staging environment
- Verify health after deployment
- Remove needs-deploy label and comment with deployment status

### Step 3b: Check application health
Use kubectl to inspect pod status and logs in your assigned namespaces:
```
kubectl get pods -n <namespace>
kubectl logs -n <namespace> <pod> --tail=50
```
If unhealthy pods found:
- Investigate logs and events
- Create a Gitea issue if a bug is found: `gitea issue create --repo {{REPOS}} --title "..." --label type/bug --label priority/high`
- Rollback if necessary

If everything is healthy: respond "No actionable items. Idle." and STOP.
