# API Codegen Design

> Related: [Control Panel Backend](control-panel-backend.md) | [Control Panel Frontend](control-panel-frontend.md)

## Overview

Establish a single-source-of-truth pipeline: Go handler annotations → OpenAPI 3.1 spec → generated TypeScript client. The frontend stops maintaining hand-written API files; instead, types and request functions are auto-generated from the backend's OpenAPI spec.

## Tool Selection

| Layer | Tool | Version | Role |
|-------|------|---------|------|
| Go → OpenAPI 3.1 | [swaggo/swag v2](https://github.com/swaggo/swag) | v2.0.0-rc5 (Jan 2026) | Parse Go annotations, emit `openapi.json` |
| OpenAPI → TypeScript | [@hey-api/openapi-ts](https://github.com/hey-api/openapi-ts) | v0.94+ (Mar 2026) | Generate axios client + TypeScript types |

**Why these tools:**
- swag v2 is the only code-first Go tool that outputs OpenAPI 3.1 natively for Gin
- @hey-api/openapi-ts is the 2026 frontrunner (used by Vercel, PayPal), with built-in axios support since v0.73.0 and a plugin-based architecture

## Pipeline

```
 ┌─────────────────────┐
 │  Go Handler Code    │  Annotated with @Summary, @Param, @Success, etc.
 │  (internal/backend/) │
 └────────┬────────────┘
          │  make openapi
          │  (swag init --v3.1)
          ▼
 ┌─────────────────────┐
 │  docs/openapi.json  │  OpenAPI 3.1 spec (committed to repo)
 └────────┬────────────┘
          │  make api-client
          │  (npx @hey-api/openapi-ts)
          ▼
 ┌─────────────────────┐
 │  frontend/src/api/  │  Generated: client.gen.ts, types.gen.ts, sdk.gen.ts
 │  generated/         │
 └─────────────────────┘
```

The generated `openapi.json` is committed to the repo so frontend developers can regenerate the client without running the Go toolchain.

## Backend: Swag Annotations

### General API Info

Add to `cmd/backend/main.go`:

```go
// @title           EntropyGen Control Panel API
// @version         1.0
// @description     AI DevOps simulation platform control panel backend.

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT token with "Bearer " prefix

// @servers.url /api
// @servers.description Default API base path
```

### Handler Annotation Pattern

Each handler method gets annotations. Example for `AgentHandler.List`:

```go
// List returns all agent CRs with enriched audit data.
// @Summary      List agents
// @Description  Returns all agent CRs enriched with token usage and current task from ClickHouse/observer.
// @Tags         agents
// @Produce      json
// @Success      200  {object}  SuccessResponse{data=[]agentapi.Agent}
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /agents [get]
func (h *AgentHandler) List(c *gin.Context) { ... }
```

### Response Type Extraction

Currently, handlers use anonymous structs and `gin.H{}` for responses. To make swag generate accurate types, we need to define named response types.

**Shared response wrapper** (new file `internal/backend/handler/response.go`):

```go
package handler

// SuccessResponse is the standard success envelope.
type SuccessResponse struct {
    Success bool        `json:"success" example:"true"`
    Data    interface{} `json:"data,omitempty"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
    Success bool   `json:"success" example:"false"`
    Error   string `json:"error"`
    Code    string `json:"code"`
    Detail  string `json:"detail,omitempty"`
}
```

**Request DTOs** — extract from anonymous structs into named types (new file `internal/backend/handler/dto.go`):

```go
package handler

// CreateAgentRequest is the request body for POST /agents.
type CreateAgentRequest struct {
    Name string          `json:"name" binding:"required"`
    Spec json.RawMessage `json:"spec" binding:"required"`
}

// AssignIssueRequest is the request body for POST /agents/:name/assign-issue.
type AssignIssueRequest struct {
    Repo     string   `json:"repo" binding:"required" example:"ai-team/webapp"`
    Title    string   `json:"title" binding:"required"`
    Body     string   `json:"body" binding:"required"`
    Labels   []string `json:"labels"`
    Priority string   `json:"priority" example:"medium"`
}

// LoginRequest is the request body for POST /auth/login.
type LoginRequest struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required"`
}

// LoginResponse is the response body for POST /auth/login.
type LoginResponse struct {
    Token    string `json:"token"`
    Username string `json:"username"`
    Role     string `json:"role"`
}

// CreateUserRequest is the request body for POST /users.
type CreateUserRequest struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required"`
    Role     string `json:"role" binding:"required" example:"member"`
}

// UpdateUserRequest is the request body for PUT /users/:username.
type UpdateUserRequest struct {
    Role     *string `json:"role,omitempty"`
    Password *string `json:"password,omitempty"`
}

// CreateModelRequest is the request body for POST /llm/models.
type CreateModelRequest struct {
    Name     string `json:"name"`
    Provider string `json:"provider"`
    APIKey   string `json:"apiKey"`
    BaseURL  string `json:"baseUrl,omitempty"`
    RPM      int    `json:"rpm"`
    TPM      int    `json:"tpm"`
}

// AssignIssueResponse is the data field for POST /agents/:name/assign-issue.
type AssignIssueResponse struct {
    IssueNumber int    `json:"issue_number"`
    IssueURL    string `json:"issue_url"`
}
```

### Endpoints NOT Annotated

These endpoints are excluded from OpenAPI generation (WebSocket/proxy, not suitable for REST client codegen):

| Endpoint | Reason |
|----------|--------|
| `WS /api/ws/events` | WebSocket, not REST |
| `ANY /api/agents/:name/observe/*path` | Reverse proxy to sidecar |

## Frontend: @hey-api/openapi-ts Configuration

### Config File

`frontend/openapi-ts.config.ts`:

```typescript
import { defineConfig } from '@hey-api/openapi-ts';

export default defineConfig({
  input: '../docs/openapi.json',
  output: {
    path: 'src/api/generated',
    lint: false,
  },
  plugins: [
    {
      name: '@hey-api/client-axios',
      runtimeConfigPath: './src/api/client-config.ts',
    },
    '@hey-api/sdk',
    '@hey-api/typescript',
  ],
});
```

### Client Configuration

`frontend/src/api/client-config.ts` — bridges the generated client with the existing axios interceptors:

```typescript
import type { CreateClientConfig } from '@hey-api/client-axios';

export const createClientConfig: CreateClientConfig = () => ({
  baseURL: '/api',
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
});
```

`frontend/src/api/setup.ts` — one-time setup called at app startup:

```typescript
import { client } from './generated/client.gen';

export function setupApiClient() {
  client.instance.interceptors.request.use((config) => {
    const token = localStorage.getItem('jwt_token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  });

  client.instance.interceptors.response.use(
    (response) => response,
    (error) => Promise.reject(error),
  );
}
```

### Migration: Old → Generated

| Old file | Replacement |
|----------|-------------|
| `src/api/client.ts` | `src/api/setup.ts` (interceptors only) |
| `src/api/agents.ts` | `import { listAgents, createAgent, ... } from './generated/sdk.gen'` |
| `src/api/auth.ts` | `import { login, logout, getMe } from './generated/sdk.gen'` |
| `src/api/llm.ts` | `import { listModels, createModel, ... } from './generated/sdk.gen'` |
| `src/api/audit.ts` | `import { listTraces, tokenUsage, ... } from './generated/sdk.gen'` |
| `src/api/roles.ts` | `import { listRoles, createRole, ... } from './generated/sdk.gen'` |
| `src/api/config.ts` | `import { getConfig } from './generated/sdk.gen'` |
| `src/api/monitor.ts` | `import { getTokenTrend, ... } from './generated/sdk.gen'` |
| `src/api/observe.ts` | Keep as-is (WebSocket/proxy, not generated) |

The `mapAgent()` function in `agents.ts` that transforms K8s CRD format to frontend types will move to a thin wrapper around the generated SDK call, since the backend response shape is now typed by the spec.

### Generated Output Structure

```
frontend/src/api/generated/
├── client.gen.ts    # Axios client instance with setConfig()
├── types.gen.ts     # All TypeScript types from OpenAPI schemas
└── sdk.gen.ts       # One function per endpoint (listAgents, createAgent, etc.)
```

This directory is `.gitignore`-excluded but can be regenerated from the committed `openapi.json`.

## Build Integration

### Makefile Targets

```makefile
# Generate OpenAPI 3.1 spec from Go annotations
openapi:
	cd internal/backend && swag init \
		--v3.1 \
		-g ../../cmd/backend/main.go \
		-d .,../../internal/common/models,../../internal/operator/api \
		-o ../../docs \
		--outputTypes json,yaml \
		--parseInternal

# Generate TypeScript client from OpenAPI spec
api-client:
	cd frontend && npx @hey-api/openapi-ts

# Combined: regenerate everything
api: openapi api-client
```

### CI Check

Add a CI step that runs `make openapi` and checks `git diff --exit-code docs/openapi.json`. If the spec file changes, the PR must include the updated spec. This catches annotation changes that forgot to regenerate.

Similarly, `make api-client` + diff check ensures the generated TypeScript stays in sync (if we choose to commit generated files) OR the CI just regenerates and builds to verify compilation.

**Chosen approach**: commit `docs/openapi.json` (small, reviewable), do NOT commit `frontend/src/api/generated/` (add to `.gitignore`). The frontend build step runs `npx @hey-api/openapi-ts` before `vite build`.

### Frontend Build Script Update

`frontend/package.json`:

```json
{
  "scripts": {
    "generate": "openapi-ts",
    "prebuild": "npm run generate",
    "dev": "npm run generate && vite",
    "build": "npm run generate && tsc && vite build"
  }
}
```

## Scope

### In Scope
- Add swag v2 annotations to all REST handler methods (~40 endpoints)
- Extract anonymous request/response structs into named DTOs
- Add `response.go` and `dto.go` files
- Configure @hey-api/openapi-ts with axios plugin
- Migrate all 7 hand-written API files to use generated SDK
- Update frontend components that import from old API files
- Add `make openapi` and `make api-client` targets
- Add generated directory to `.gitignore`

### Out of Scope
- WebSocket endpoints (ws/events) — remain hand-written
- Observer proxy endpoint — remains hand-written
- Swagger UI serving (can be added later if wanted)
- API versioning (current API has no version prefix beyond `/api/`)

## Risks

| Risk | Mitigation |
|------|------------|
| swag v2 is RC, not stable | rc5 is mature enough; pin exact version in `go install` |
| Anonymous `gin.H{}` responses not captured by swag | Extract to named structs in `response.go` / `dto.go` |
| Frontend `mapAgent()` transform logic lost in migration | Keep as wrapper function that post-processes generated SDK calls |
| Generated function names may not match current usage | Configure `@hey-api/sdk` naming or add re-export aliases |
