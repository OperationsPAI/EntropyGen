# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Language Rules

- All code, comments, commit messages, design documents, and file content: **English**
- Communication with the user (conversation, explanations, questions): **Chinese**

## Current Phase: Architecture Design

The project is in the early design phase. All outputs are currently markdown documents, **code writing is prohibited**. All component designs must be confirmed by the user before finalization.

## Build & Development Commands

This project uses **uv** as the package manager and build tool for python.

```bash
uv sync              # Install dependencies
uv run agentm        # Run the CLI entry point
uv add <package>     # Add a dependency
uv run pytest        # Run tests
uv run pytest tests/path/to/test_file.py::test_name  # Run a single test
```

Use golang as the implementation language for backend component. 

## Design Documentation System

All design documents are stored in the `.claude/` directory. A `.claude/index.yaml` file maintains the conceptual index and relationships between concepts.

### Directory Structure

```
.claude/
├── index.yaml          # Concept index and relationship graph, must always stay up-to-date
├── designs/            # High-level design documents (continuously maintained, updated with changes)
├── plans/              # Complete implementation plans (append-only, never modify existing files)
└── tasks/              # Subtasks within plans (append-only, never modify existing files)
```

### File Rules

| Directory | Lifecycle | Naming Convention | Description |
|-----------|-----------|-------------------|-------------|
| `designs/` | **Continuously maintained** | `<concept>.md` | Top-level design; must sync updates when concepts change |
| `plans/` | **Append-only** | `YYYY-MM-DD-<plan-name>.md` | Complete implementation plan |
| `tasks/` | **Append-only** | `YYYY-MM-DD-<task-name>.md` | Breakdown of subtasks under a plan |

### index.yaml

`index.yaml` is the relationship graph for the entire design system, recording references between each concept and related documents.

**Structure Convention:**

```yaml
concepts:
  <concept-name>:
    description: "One-sentence description"
    design: "designs/<file>.md"        # Corresponding design document
    related_concepts: [<other-concept>] # Related concepts
    plans: ["plans/<file>.md"]          # Related plans
    tasks: ["tasks/<file>.md"]          # Related tasks
```

**Maintenance Rules:**

1. When adding design/plan/task files, must sync update `index.yaml`
2. When a concept changes, trace `related_concepts` links in `index.yaml` to find all affected documents and update them
3. If changes impact documents in `designs/`, must update the design documents

### Cross-Referencing

All markdown documents link to each other via relative paths:

- Design references other designs: `[concept-name](other-concept.md)`
- Plan references design: `[design-doc](../designs/concept.md)`
- Task references plan: `[plan](../plans/YYYY-MM-DD-plan.md)`
- Task references design: `[design](../designs/concept.md)`

### Change Propagation Workflow

When any design concept changes, execute the following workflow:

1. **Update design document** — Modify the corresponding file in `designs/`
2. **Query index.yaml** — Find all related concepts in `related_concepts`
3. **Assess impact scope** — Determine if related design documents need updating
4. **Update affected documents** — Sync changes to all affected design files
5. **Update index.yaml** — If adding/removing concepts or relationships, update the index
6. **Create new plan/task** — If changes need an implementation plan, append new files in `plans/` and `tasks/` (never modify existing files)

## Testing Philosophy

Every test must be deliberate. Adding, modifying, or removing a test requires careful justification.

### Principles

1. **Test behavior, not structure** — Ask "what does this code do" not "what fields does it have". Never test language guarantees (enum values exist, dataclass defaults work, imports succeed).
2. **Every test must answer: "what bug does this prevent?"** — If you can't articulate a realistic failure scenario, don't write the test.
3. **Boundaries over happy paths** — Edge cases, invalid inputs, and state transitions are where bugs live. A single well-chosen boundary test beats ten happy-path assertions.
4. **One scenario per test, multiple asserts are fine** — Group related assertions that verify the same logical scenario. Don't split `assert a` and `assert b` into separate tests if they test the same thing.
5. **Don't test other people's code** — Pydantic validation, Python dataclass semantics, enum membership, `isinstance` checks — these are framework guarantees, not our responsibility.

### What to Test

- **State transitions and rules** — PhaseManager valid/invalid transitions
- **Immutability contracts** — Notebook operations return new instances, originals unchanged
- **Boundary conditions** — Empty inputs, missing keys, malformed paths
- **Cross-module contracts** — State registry resolves correct types, config validation rejects bad references
- **Invariants the design doc mandates** — e.g., HypothesisStatus values match the tool's Literal constraint

### What NOT to Test

- Field existence or default values on dataclasses/TypedDicts/Pydantic models
- Enum value counts or individual member existence
- Import success (`assert X is not None`)
- Type inheritance (`isinstance` checks)
- Stub functions that only `raise NotImplementedError`

## Slash Commands

Project-specific commands are located in `.claude/commands/`:

| Command | Purpose |
|---------|---------|
| `/design <concept>` | Create or update design document, maintain index.yaml relationships, propagate changes |
| `/plan` | Create implementation plan, break down into tasks, wait for user confirmation before execution |
| `/index [show\|check\|fix]` | View, verify, or fix index.yaml consistency |
| `/status` | Project status overview: design progress, plan status, index health |
| `/tdd` | TDD development workflow (use after entering implementation phase) |
| `/eval [define\|check\|report]` | Manage acceptance criteria and evaluation |
| `/checkpoint [create\|verify\|list]` | Workflow checkpoint management |
| `/learn` | Extract reusable patterns from current session |

## Agents

Project-specific agents in `.claude/agents/`:

| Agent | Role | Model | Tools | Primary Commands/Skills |
|-------|------|-------|-------|------------------------|
| **architect** | Architecture design, produce design docs, maintain index.yaml | opus | Read, Grep, Glob | `/design`, `/index` |
| **planner** | Break designs into executable plans and tasks | sonnet | Read, Grep, Glob | `/plan`, `/status` |
| **tdd** | Write tests first (RED), guide TDD cycle | sonnet | Read, Write, Edit, Bash, Grep, Glob | `/tdd`, `/eval` |
| **implementer** | Execute plans, write code following designs | sonnet | Read, Write, Edit, Bash, Grep, Glob | `/checkpoint` |
| **reviewer** | Verify implementation matches design, code quality | opus | Read, Grep, Glob, Bash | `/eval`, `/index check` |

### Agent Workflow

```
architect → planner → tdd (write tests) → implementer (write code) → reviewer
    ↑                                                                     |
    └─────────────── feedback (design issues) ────────────────────────────┘
```

### When to Use Each Agent

- **Design discussion / new concept** → architect
- **Break work into tasks** → planner
- **Write tests for a module** → tdd
- **Implement a plan task** → implementer
- **Verify completed work** → reviewer
