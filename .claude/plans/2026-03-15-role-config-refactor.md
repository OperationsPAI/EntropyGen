# Plan: Agent 配置数据流重构 — Role 为指令中心

> Date: 2026-03-15
> Status: Implemented
> Related design: [operator.md](../designs/operator.md)

## Summary

Refactored the agent configuration data flow to establish a clear two-layer model:
- **Role (指令层)**: SOUL.md, AGENTS.md, PROMPT.md, Skills — defines *what* the agent does
- **Agent Spec (运行时层)**: model, temperature, cron schedule, resources, paused — defines *how* the agent runs

## Changes Made

### Phase 1: Builtin Role Templates
- Created `agent-runtime/builtin-role/` with default templates:
  - `SOUL.md` — default agent identity
  - `PROMPT.md` — default cron prompt
  - `agents-templates/base.md` — common behavior constraints
  - `agents-templates/{developer,reviewer,sre,observer}.md` — role-specific behavior

### Phase 2: Backend Role Creation Injects Builtin Content
- Created `internal/backend/builtin/` package with Go embed for templates and skills
- Added `BuiltinContentProvider` interface in `k8sclient` package
- Updated `CreateRoleRequest` with optional `role` field
- Updated `RoleClient.Create()` to inject builtin SOUL.md, PROMPT.md, AGENTS.md, and skills
- Updated `build/backend/Dockerfile` to COPY builtin files at build time
- Updated frontend `CreateRoleDto` with optional `role` field
- Updated `NewRole.tsx` with Role Type selector and skills preview

### Phase 3: Operator Simplification
- Removed `//go:embed skills/**/SKILL.md` from operator
- Removed `readSkill()`, `buildAgentsMD()` functions
- Simplified `buildConfigMapData()` — Role data is the single source of truth
- Simplified `buildSkillsData()` — skills come entirely from Role
- Simplified `CronPrompt()` — reads only from Role PROMPT.md
- Removed `spec.cron.prompt` field from `AgentCron` struct
- Removed `internal/operator/reconciler/skills/` directory
- Rewrote all tests to match new behavior

### Phase 4: Role ConfigMap Watch
- Added `Watches()` for role-* ConfigMaps in `SetupWithManager()`
- Implemented `isRoleConfigMap` predicate (checks `entropygen.io/component=role` label)
- Implemented `mapRoleToAgents` fan-out function

### Phase 5: Example YAML Cleanup
- Removed `spec.cron.prompt` from all `k8s/examples/*.yaml`

## Verification
- `go build ./...` — passes
- `go test ./internal/operator/...` — 12/12 tests pass
- `go test ./internal/backend/...` — all tests pass
- Frontend TypeScript check — passes
