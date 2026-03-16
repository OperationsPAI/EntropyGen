# Plan: Agent Permission Editing + Operator Permission Fix

## Context

Agent permissions (Gitea repo + read/write/review/merge) are configured during creation but:
1. **Cannot be edited after creation** — no UI exists on the Detail page
2. **Never actually applied** — the operator stores permissions in the CRD spec but never calls `AddCollaborator()` to set them on Gitea repos

This causes the reported error: agent user has `pull` but not `push` permission.

## Changes

### 1. Operator: Add `EnsureGiteaRepoAccess` reconciler step

**File: `internal/operator/reconciler/gitea_user.go`**

Add two functions:

- `resolveGiteaAccessMode(permissions []string) string` — maps frontend permissions to Gitea access mode:
  - Any of `write`/`review`/`merge` present → `"write"`
  - Otherwise → `"read"`
- `EnsureGiteaRepoAccess(ctx, agent)` — for each repo in `agent.Spec.Gitea.Repos`, calls `r.GiteaClient.AddCollaborator(ctx, owner, repo, username, accessMode)`. Skips if no repos or no gitea client. Handles invalid repo format (no `/`) with a log warning.

**File: `internal/operator/reconciler/resource_reconciler.go`**

Insert `{"gitea-repo-access", r.EnsureGiteaRepoAccess}` as step 3 in `ReconcileAll` (after gitea-token, before jwt-secret). `AddCollaborator` uses the admin token (not the agent token), so it only needs the user to exist.

### 2. Frontend API: Fix HTTP method mismatch + body format

**File: `frontend/src/api/agents.ts`**

- Change `apiClient.patch` → `apiClient.put` (backend registers `PUT`, not `PATCH`)
- Send spec directly as body (backend's `Update` handler reads raw body as spec, not wrapped in `{spec:...}`)

**File: `frontend/src/types/agent.ts`**

- Remove `UpdateAgentDto` wrapper — `updateAgent` takes `AgentSpec` directly

### 3. Frontend API: Fix `mapAgent` repo field mapping

**File: `frontend/src/api/agents.ts`** (mapAgent function, line 34)

CRD returns `spec.gitea.repos` (array), but mapAgent reads `spec.gitea?.repo` (undefined for CRD format). Fix to:
```
repo: spec.gitea?.repo ?? (spec.gitea?.repos?.[0] ?? ''),
```

### 4. Frontend: Add "Edit Permissions" modal to Detail page

**File: `frontend/src/pages/Agents/Detail.tsx`**

Follow the existing Assign Task modal pattern:

- **State**: `permModal`, `permLoading`, `permForm: {repo, permissions}`
- **Button**: "Edit Permissions" in sidebar, resets form from current `agent.spec.gitea` on open
- **Handler**: `handleUpdatePermissions` — sends full `agent.spec` with updated gitea config via `agentsApi.updateAgent(name, updatedSpec)`, updates local agent state on success
- **Modal**: repo Input + permissions checkboxes (read/write/review/merge), matching NewAgent.tsx pattern

Also add Repo + Permissions display rows in Overview grid.

## Files to Modify

| File | Change |
|------|--------|
| `internal/operator/reconciler/gitea_user.go` | Add `EnsureGiteaRepoAccess()` + `resolveGiteaAccessMode()` |
| `internal/operator/reconciler/resource_reconciler.go` | Add step to `ReconcileAll` |
| `frontend/src/api/agents.ts` | Fix PATCH→PUT, send spec directly, fix mapAgent repo mapping |
| `frontend/src/types/agent.ts` | Update `UpdateAgentDto` |
| `frontend/src/pages/Agents/Detail.tsx` | Add Edit Permissions modal + overview display |

## Verification

1. `skaffold run` to build and deploy
2. Create a new agent with repo `platform/microservices-demo` + permissions `[read, write]`
3. Check Gitea: agent user should be added as collaborator with `write` access
4. Open Agent Detail → verify Repo + Permissions shown in Overview
5. Click "Edit Permissions" → change permissions → save
6. Verify operator re-reconciles and updates Gitea collaborator access
