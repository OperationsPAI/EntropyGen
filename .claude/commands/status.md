---
description: Display project status overview: design progress, plan status, index health.
---

# Status Command

Quick view of overall project status.

## Output

```
AgentM Project Status
====================

## Design Documents
- N concepts defined
- X DRAFT / Y APPROVED
- List each concept and status

## Plans
- Latest plan: YYYY-MM-DD-xxx (status)
- Active tasks: N

## Index Health
- Concept count: N
- Relationship completeness: ✓/✗
- Orphaned documents: N

## Git
- Current branch: xxx
- Uncommitted changes: N files
```

## Workflow

1. Read `.claude/index.yaml`
2. Scan `designs/`, `plans/`, `tasks/` directories
3. Read status field from each design document
4. Check index consistency
5. Get git status
6. Display summary
