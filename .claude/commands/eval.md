---
description: Manage acceptance criteria and evaluation. Define, check, report feature completion status.
---

# Eval Command

Manage acceptance criteria and evaluation workflow for features.

## Usage

- `/eval define <feature>` — Define acceptance criteria
- `/eval check <feature>` — Check current status
- `/eval report <feature>` — Generate complete report
- `/eval list` — List all eval definitions

## Define Acceptance Criteria

Create `.claude/evals/<feature>.md`:

```markdown
## EVAL: <feature>
Date Created: YYYY-MM-DD

### Capability Verification
- [ ] Feature point description 1
- [ ] Feature point description 2

### Regression Verification
- [ ] Existing behavior 1 not affected
- [ ] Existing behavior 2 not affected

### Pass Criteria
- Capability verification: pass@3 > 90%
- Regression verification: pass^3 = 100%
```

## Check Status

```
EVAL CHECK: <feature>
========================
Capability verification: X/Y passed
Regression verification: X/Y passed
Status: IN_PROGRESS / READY
```

## Report

```
EVAL REPORT: <feature>
=========================
Capability verification: [PASS/FAIL details]
Regression verification: [PASS/FAIL details]
Recommendation: SHIP / NEEDS_WORK / BLOCKED
```

## Verification Commands (after entering development phase)

```bash
# Run tests
uv run pytest

# Coverage check
uv run pytest --cov=agentm --cov-report=term-missing

# Type check
uv run mypy src/

# Code check
uv run ruff check src/
```
