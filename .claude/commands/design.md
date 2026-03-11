---
description: Create or update design documents. Maintain index.yaml relationships, automatically propagate changes to related documents.
---

# Design Command

Manage design documents under `.claude/designs/`, maintaining relationships between concepts.

## Usage

`/design <concept-name>` — Create or update a design document for a concept

## Workflow

### Creating a New Design

1. **Confirm the concept** — Align with user on concept definition and scope
2. **Create document** — Write design in `.claude/designs/<concept>.md`
3. **Update index** — Register concept and relationships in `.claude/index.yaml`
4. **Cross-reference** — Add links in related design documents

### Updating an Existing Design

1. **Read index** — Get concept relationship graph from `index.yaml`
2. **Update document** — Modify `.claude/designs/<concept>.md`
3. **Propagate changes** — Check and update affected documents via `related_concepts`
4. **Sync index** — Update relationships in `index.yaml`

## Design Document Template

```markdown
# Design: <Concept Name>

**Status**: DRAFT | REVIEW | APPROVED
**Created**: YYYY-MM-DD
**Last Updated**: YYYY-MM-DD

## Overview
[One-sentence description of this concept's responsibility]

## Motivation
[Why is this concept needed, what problem does it solve]

## Design Details
[Core design content]

## Interface Definition
[Interfaces, protocols, contracts exposed externally]

## Related Concepts
- [Concept A](concept-a.md) — Relationship description
- [Concept B](concept-b.md) — Relationship description

## Constraints and Decisions
| Decision | Rationale | Alternative |
|----------|-----------|-------------|
| ... | ... | ... |

## Open Questions
- [ ] Issues to discuss
```

## Change Propagation Rules

When modifying a design document, you must:

1. Read `related_concepts` for that concept in `index.yaml`
2. Check if each related concept's document needs updating
3. If related concept's interface, constraints, or assumptions are affected, update that document
4. Record change in `index.yaml`

## Notes

- Design documents are **continuously maintained**, evolving with concepts
- All design changes must be confirmed by user
- Code writing is prohibited in the design phase; design documents contain only markdown
