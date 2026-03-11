---
description: View, verify, or fix index.yaml concept index consistency.
---

# Index Command

Manage `.claude/index.yaml` concept index.

## Usage

- `/index show` — Display current index overview
- `/index check` — Verify index and document consistency
- `/index fix` — Fix inconsistencies in the index

## Show

Display all concepts and their relationships in readable format:

```
Concept Index Overview
======================
<concept-1>
  Design: designs/concept-1.md
  Related: concept-2, concept-3
  Plans: 2
  Tasks: 5

<concept-2>
  Design: designs/concept-2.md
  Related: concept-1
  Plans: 1
  Tasks: 3
```

## Check

Verify consistency:

1. Do all files referenced in `index.yaml` exist?
2. Are all files in `designs/` registered in `index.yaml`?
3. Are all files in `plans/` and `tasks/` linked?
4. Are all concepts in `related_concepts` defined?
5. Are all cross-reference links in documents valid?

```
INDEX CHECK
===========
✓ All referenced files exist
✗ designs/orphan.md not registered in index
✗ concept-x's related_concepts references undefined concept-y
```

## Fix

Fix inconsistencies based on check results:

- Register orphaned files in the index
- Remove references to non-existent files
- Complete missing bidirectional relationships
- Require user confirmation before fixing
