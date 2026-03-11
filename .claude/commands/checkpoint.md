---
description: Create or verify workflow checkpoints, record current state.
---

# Checkpoint Command

Create checkpoints during design or development workflow, recording progress.

## Usage

- `/checkpoint create <name>` — Create a named checkpoint
- `/checkpoint verify <name>` — Compare current state with checkpoint
- `/checkpoint list` — List all checkpoints

## Create Checkpoint

1. Check current state (git status, design document completeness)
2. Create git commit or stash
3. Record to `.claude/checkpoints.log`:

```
YYYY-MM-DD-HH:MM | <name> | <git-sha> | <notes>
```

## Verify Checkpoint

Compare current state with a checkpoint:

```
CHECKPOINT: <name>
====================
File changes: X
Design documents: Y new/modified
index.yaml: synced/out-of-sync
Git: <commit difference>
```

## Applicable Scenarios

- After design discussions reach incremental consensus
- After plan confirmation, before starting execution
- Before major changes, as a safety point
