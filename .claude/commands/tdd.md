---
description: Enforce test-driven development workflow. Write interface first, then tests, then minimal implementation. Ensure 80%+ coverage.
---

# TDD Command

Test-driven development using pytest.

## TDD Cycle

```
RED → GREEN → REFACTOR → REPEAT

RED:      Write a failing test
GREEN:    Write minimal code to pass the test
REFACTOR: Improve code while keeping tests passing
REPEAT:   Next scenario
```

## Workflow

1. **Define interface** — Create types and interfaces (Python protocol/dataclass/TypedDict)
2. **Write failing test** — pytest test file, confirm test fails for the right reason
3. **Minimal implementation** — Write only enough code to pass the test
4. **Refactor** — Improve code quality while keeping tests green
5. **Check coverage** — Ensure 80%+ coverage

## Command Reference

```bash
# Run all tests
uv run pytest

# Run single test file
uv run pytest tests/test_xxx.py

# Run single test function
uv run pytest tests/test_xxx.py::test_function_name

# With coverage
uv run pytest --cov=agentm --cov-report=term-missing

# Only run failing tests
uv run pytest --lf

# Verbose output
uv run pytest -v
```

## Test File Structure

```
tests/
├── conftest.py          # Shared fixtures
├── unit/                # Unit tests
│   └── test_xxx.py
├── integration/         # Integration tests
│   └── test_xxx.py
└── e2e/                 # End-to-end tests
    └── test_xxx.py
```

## Coverage Requirements

- **80% minimum** — All code
- **100% required** — Core business logic, security-related code

## Prerequisites

Ensure pytest dependencies are added to `pyproject.toml`:

```bash
uv add --dev pytest pytest-cov pytest-asyncio
```

## Next Commands

- `/plan` — Understand what to build first
- `/checkpoint` — Create checkpoint after tests pass
