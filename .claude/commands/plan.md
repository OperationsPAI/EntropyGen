---
description: Create implementation plans. Restate requirements, assess risks, break down steps. Must wait for user confirmation before execution.
---

# Plan Command

Create a complete implementation plan, output to `.claude/plans/YYYY-MM-DD-<name>.md` file.

## Workflow

1. **Restate requirements** — Describe the goal in your own words, ensure understanding aligns
2. **Identify risks** — List potential issues and blocking items
3. **Break down steps** — Phased, executable implementation plan
4. **Link designs** — Reference related design documents in `.claude/designs/`
5. **Wait for confirmation** — Proceed only after explicit user agreement

## Outputs

- Plan file: `.claude/plans/YYYY-MM-DD-<plan-name>.md`
- Task files: `.claude/tasks/YYYY-MM-DD-<task-name>.md` (one per step)
- Update relationships in `.claude/index.yaml`

## Plan Template

```markdown
# Plan: <Title>

**Date**: YYYY-MM-DD
**Status**: DRAFT | CONFIRMED | IN_PROGRESS | DONE

## Requirements Restatement
[Describe the goal in your own words]

## Related Designs
- [Design Document](../designs/xxx.md)

## Implementation Phases

### Phase 1: <Name>
- Step description
- Corresponding task: [task](../tasks/YYYY-MM-DD-xxx.md)

### Phase 2: <Name>
...

## Risk Assessment
| Risk | Level | Mitigation |
|------|-------|-----------|
| ... | HIGH/MEDIUM/LOW | ... |

## Dependencies
- [List prerequisites and external dependencies]
```

## Use Cases

- Starting a new feature
- Major architectural changes
- Complex refactoring
- When requirements are unclear and alignment is needed

## Notes

- **Current phase is design; code writing is prohibited**. Plan output is markdown documents only
- Plan files are append-only; create new files when changing plans
- Must register relationships in `index.yaml`

## Next Commands

- `/design` — Create or update design documents
- `/tdd` — Use TDD development after entering implementation phase
- `/checkpoint` — Create checkpoints
