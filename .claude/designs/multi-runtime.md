# Multi-Runtime Agent Architecture

> Related overview: [System Design Overview](system-design-overview.md)
> Related: [Agent Runtime](agent-runtime.md) | [Operator](operator.md) | [Control Panel Frontend](control-panel-frontend.md)

## 1. Overview

The current platform is tightly coupled to OpenClaw as the sole agent runtime. This design introduces a **Runtime Adapter** abstraction that allows any AI agent framework to be plugged into the platform as a black box.

**Core principle**: The platform treats each agent runtime as a black box. From the platform's perspective, an agent:
- Accepts tasks from Gitea (Issue Board as task queue)
- Completes tasks via Gitea (PRs, comments)
- Uses LLM APIs through the platform's LiteLLM proxy

The platform manages lifecycle (start/stop/health), injects configuration, and observes behavior — without knowing what framework runs inside.

## 2. Runtime Contract

The contract defines the interface between the platform and any agent runtime adapter. Any framework that satisfies this contract can be integrated.

### 2.1 Environment Variables

The platform injects all agent context via environment variables. The adapter decides how to use them.

| Variable | Description | Example |
|----------|-------------|---------|
| `AGENT_ID` | Unique agent identifier | `agent-developer-1` |
| `AGENT_ROLE` | Role name | `developer` |
| `LLM_BASE_URL` | LiteLLM API endpoint | `http://litellm.llm-gateway.svc:4000/v1` |
| `LLM_API_KEY` | LiteLLM API key | `sk-...` |
| `LLM_MODEL` | Default model identifier | `anthropic/claude-sonnet-4-20250514` |
| `GITEA_BASE_URL` | Gitea URL | `http://gitea.devops-infra.svc:3000` |
| `GITEA_TOKEN_FILE` | Path to Gitea token file | `/agent/secrets/gitea-token` |
| `GITEA_REPOS` | Repos to work on (comma-separated) | `org/platform-demo` |
| `WORKSPACE_DIR` | Workspace root directory | `/workspace` |

### 2.2 File Mounts

| Path | Type | Description |
|------|------|-------------|
| `/agent/secrets/gitea-token` | Secret (read-only) | Gitea API token |
| `/agent/prompt/` | ConfigMap (read-only) | Role prompt files (SOUL.md, AGENTS.md, PROMPT.md). Adapter decides how to use them |
| `/agent/skills/` | ConfigMap (read-only) | Role skill files. Adapter decides how to use them |
| `/workspace/` | PVC (read-write) | Persistent workspace. Platform monitors changes in this directory |

### 2.3 Behavioral Contract

| Contract | Requirement |
|----------|-------------|
| **Health check** | Expose `GET :8080/healthz`, return 200 when alive |
| **Task source** | Autonomously poll Gitea Issue Board for tasks |
| **Result output** | Deliver results via Gitea PRs/Comments |
| **LLM calls** | Must use `LLM_BASE_URL`, no direct external LLM access |
| **Workspace** | All code operations happen under `/workspace/` |
| **Logging** | stdout/stderr structured logs (JSON recommended) |

## 3. Agent CRD Changes

### 3.1 New `spec.runtime` Field

```yaml
apiVersion: aidevops.io/v1
kind: Agent
metadata:
  name: developer-1
spec:
  role: developer

  # New: runtime configuration
  runtime:
    type: openclaw          # Runtime type identifier, selects adapter image
    image: ""               # Optional override. Empty = auto-select by type
    env: []                 # Optional extra env vars (adapter-specific config)

  # Retained: generic fields
  llm:
    model: "anthropic/claude-sonnet-4-20250514"
  gitea:
    repos: ["org/platform-demo"]
  resources:
    cpu: "500m"
    memory: "512Mi"
  memory:
    storageSize: "1Gi"
  paused: false
```

### 3.2 Runtime Type → Image Registry

The Operator maintains an internal registry mapping `runtime.type` to default images:

```go
var runtimeImages = map[string]string{
    "openclaw":    "registry.local/agent-runtime-openclaw:latest",
    "claude-code": "registry.local/agent-runtime-claude-code:latest",
    "aider":       "registry.local/agent-runtime-aider:latest",
    // Adding a new framework = one line here
}
```

If `spec.runtime.image` is non-empty, it takes priority (full customization).

### 3.3 Removed Fields

| Original Field | Handling |
|----------------|----------|
| `spec.cron.schedule` / `spec.cron.prompt` | Moved into Role prompt files or `runtime.env`, handled by adapter |
| `spec.runtimeImage` | Replaced by `spec.runtime.image` |

### 3.4 Backward Compatibility

- `spec.runtime` defaults to `{type: "openclaw"}` — existing Agent CRs work without modification
- CRs without `runtime` field automatically fall back to openclaw

## 4. Operator Changes

### 4.1 Core Change: Remove OpenClaw-Specific Logic

**Before** (current):
```
Operator → generate openclaw.json + SOUL.md + AGENTS.md → ConfigMap → Pod
```

**After**:
```
Operator → mount raw Role files to /agent/prompt/ → inject generic env vars → Pod
         → adapter entrypoint.sh converts Role files to framework-specific format
```

### 4.2 ConfigMap Simplification

No more `openclaw.json` generation. ConfigMaps only store raw Role files:

| ConfigMap | Content | Change |
|-----------|---------|--------|
| `agent-{name}-prompt` | SOUL.md, AGENTS.md, PROMPT.md (raw) | Renamed from `agent-{name}-config`, removed openclaw.json |
| `agent-{name}-skills` | skills/** | No change |
| `agent-{name}-role-files` | Extra files | No change |

### 4.3 Deployment Generation

```go
// Image selection
func agentRuntimeImage(agent *agentapi.Agent) string {
    if agent.Spec.Runtime.Image != "" {
        return agent.Spec.Runtime.Image       // User override takes priority
    }
    if img, ok := runtimeImages[agent.Spec.Runtime.Type]; ok {
        return img                             // Lookup by type
    }
    return runtimeImages["openclaw"]           // Default fallback
}

// Environment variables: all generic
envVars := []corev1.EnvVar{
    {Name: "AGENT_ID", Value: "agent-" + agent.Name},
    {Name: "AGENT_ROLE", Value: agent.Spec.Role},
    {Name: "LLM_BASE_URL", Value: llmBaseURL},
    {Name: "LLM_API_KEY", Value: llmAPIKey},
    {Name: "LLM_MODEL", Value: agent.Spec.LLM.Model},
    {Name: "GITEA_BASE_URL", Value: giteaURL},
    {Name: "GITEA_TOKEN_FILE", Value: "/agent/secrets/gitea-token"},
    {Name: "GITEA_REPOS", Value: strings.Join(agent.Spec.Gitea.Repos, ",")},
    {Name: "WORKSPACE_DIR", Value: "/workspace"},
}
// Append adapter-specific env vars
envVars = append(envVars, agent.Spec.Runtime.Env...)
```

### 4.4 Mount Changes

```
/agent/prompt/          ← ConfigMap agent-{name}-prompt (read-only)
    ├── SOUL.md
    ├── AGENTS.md
    └── PROMPT.md
/agent/skills/          ← ConfigMap agent-{name}-skills (read-only)
/agent/role/            ← ConfigMap agent-{name}-role-files (read-only, optional)
/agent/secrets/         ← Secrets (read-only)
/workspace/             ← PVC (read-write)
```

Workspace mount point changes from `/home/node/.openclaw/workspace` to the generic `/workspace/`.

### 4.5 Unchanged Parts

- Gitea user creation/deletion, Finalizer mechanism
- ServiceAccount / RoleBinding
- Config hash rolling restart (hash now computed on prompt ConfigMap)
- Role PVC scanning and change propagation

## 5. Observer Changes

### 5.1 Workspace Monitoring (Replaces Session File Monitoring)

Monitor `/workspace/` directory for file changes, reflecting the agent's actual operations:

```
Observer monitors /workspace/
  │
  ├── Filesystem Watcher (inotify / fsnotify)
  │   → Detect file create, modify, delete events
  │   → Push to WebSocket Hub
  │
  └── Git Diff Polling (every 10 seconds)
      → Run git diff / git status in /workspace/{repo}/
      → Generate structured change summaries
      → Push to WebSocket Hub
```

**Frontend display**: From "live session stream" to "live workspace change feed" — see what files the agent created, what code changed, what commits were made. Essentially like watching a developer's live screen share.

### 5.2 Conversation Records (From LiteLLM)

No longer read from framework internal files. Query LiteLLM logs instead:

```
Frontend requests agent conversation history
  → Backend API
  → Query LiteLLM spend logs / callback data
     Filter: metadata.agent_id = "agent-developer-1"
  → Return time-sorted request/response records
```

**Required LiteLLM configuration**:
- Each agent injects `agent_id` in request metadata when calling LiteLLM
- LiteLLM enables log persistence (existing ClickHouse or PostgreSQL can be reused)

**Adapter-side support**: Adapters inject `agent_id` into LLM API requests via:
- Frameworks with custom HTTP header support — configure directly
- Frameworks without — deploy a lightweight HTTP proxy in the image (e.g., a few lines of nginx config) that auto-appends the header to all `LLM_BASE_URL` requests

### 5.3 New Observer Architecture

```
Observer (in-pod sidecar, unchanged role)
  │
  ├── WorkspaceWatcher      ← Monitor /workspace/ file changes (replaces SessionWatcher)
  ├── GitDiffPoller          ← Periodic git diff for change summaries (new)
  ├── WebSocket Hub          ← Push real-time events to frontend (unchanged)
  └── HTTP API               ← GET /status, GET /workspace/files etc. (simplified)
```

**Removed**:
- `CompletionsDir` / Session JSONL logic
- OpenClaw path hardcoding (`~/.openclaw/...`)

## 6. Adapter Implementation Specification

### 6.1 Adapter Responsibilities

Each adapter is a Docker image that does three things:

1. **Read generic inputs** — environment variables + `/agent/prompt/` + `/agent/skills/`
2. **Convert to framework format** — generate framework-specific config files
3. **Start framework process** — run the agent main loop

### 6.2 OpenClaw Adapter (Refactored from Current Code)

`agent-runtime-openclaw/entrypoint.sh`:

```bash
#!/bin/bash
set -euo pipefail

# ---- Common layer: same for all adapters ----
source /common/setup-git.sh

# ---- Adapter layer: OpenClaw-specific ----
# Convert generic prompt files to OpenClaw format
cp -f /agent/prompt/* ~/.openclaw/ 2>/dev/null || true
cp -rf /agent/skills/* ~/.openclaw/skills/ 2>/dev/null || true

# Generate openclaw.json from environment variables
cat > ~/.openclaw/openclaw.json <<EOCFG
{
  "models": {
    "providers": {
      "litellm": {
        "baseUrl": "${LLM_BASE_URL}",
        "apiKey": "${LLM_API_KEY}",
        "api": "openai-completions",
        "models": [{
          "id": "${LLM_MODEL#*/}",
          "name": "${LLM_MODEL#*/}",
          "contextWindow": 128000,
          "maxTokens": 32000
        }]
      }
    }
  },
  "agents": {
    "defaults": {
      "model": { "primary": "${LLM_MODEL}" }
    }
  }
}
EOCFG

# Template variable substitution
sed -i "s|{{AGENT_ID}}|${AGENT_ID}|g" ~/.openclaw/SOUL.md

# Start Observer + OpenClaw
agent-observer &
exec openclaw gateway run --port 8080 --bind lan --allow-unconfigured
```

### 6.3 Example: Claude Code Adapter

Demonstrates how simple it is to integrate a new framework:

```dockerfile
# agent-runtime-claude-code/Dockerfile
FROM node:22-slim
RUN npm install -g @anthropic-ai/claude-code
COPY entrypoint.sh /entrypoint.sh
COPY agent-observer /usr/local/bin/agent-observer
ENTRYPOINT ["/entrypoint.sh"]
```

```bash
# agent-runtime-claude-code/entrypoint.sh
#!/bin/bash
set -euo pipefail

source /common/setup-git.sh

# Combine SOUL.md + AGENTS.md into CLAUDE.md
cd /workspace
{
  cat /agent/prompt/SOUL.md
  echo ""
  cat /agent/prompt/AGENTS.md
} > CLAUDE.md 2>/dev/null || true

TASK_PROMPT=$(cat /agent/prompt/PROMPT.md 2>/dev/null || echo "Check Gitea for tasks")

agent-observer &

while true; do
    claude --dangerously-skip-permissions -p "$TASK_PROMPT"
    sleep 120
done
```

### 6.4 Adapter Developer Checklist

To integrate a new framework:

| Step | Content |
|------|---------|
| 1 | Create `agent-runtimes/{name}/Dockerfile` — install the framework |
| 2 | Create `agent-runtimes/{name}/entrypoint.sh` — common layer + adapter layer |
| 3 | Add one line to Operator image registry |
| 4 | Write Role prompt files adapted to the framework |

No Operator code changes, no Observer changes, no frontend changes required.

### 6.5 Directory Structure

```
agent-runtimes/                # New structure
├── common/                    # Shared scripts (git config, credentials, etc.)
│   └── setup-git.sh
├── openclaw/                  # OpenClaw adapter
│   ├── Dockerfile
│   └── entrypoint.sh
├── claude-code/               # Claude Code adapter (example)
│   ├── Dockerfile
│   └── entrypoint.sh
└── template/                  # Template for new adapters
    ├── Dockerfile.template
    └── entrypoint.sh.template
```

## 7. Impact Summary

### 7.1 Components to Modify

| Component | Change | Complexity |
|-----------|--------|------------|
| **Agent CRD** | Add `spec.runtime` field, remove `spec.cron` | Low |
| **Operator reconciler** | Remove `openclaw.json` generation, generic env var injection + image selection | Medium |
| **Observer** | Remove Session JSONL monitoring, add workspace file watcher + git diff | Medium |
| **agent-runtime/** | Reorganize into `agent-runtimes/openclaw/`, split entrypoint into common/adapter layers | Low |
| **Frontend Observe page** | "Session stream" → "workspace change feed", conversation from LiteLLM | Medium |
| **LiteLLM config** | Enable log persistence, support `agent_id` queries | Low |

### 7.2 Components Unchanged

- **Agent Gateway** — JWT auth + proxy, unchanged
- **Event Collector** — Gitea Webhook collection, unchanged
- **Backend API** — Agent CRUD mostly unchanged, new `runtime` field passthrough
- **Gitea integration** — User creation/permissions, unchanged
- **Redis / ClickHouse data pipeline** — unchanged

### 7.3 Backward Compatibility

- `spec.runtime` defaults to `{type: "openclaw"}` — existing Agent CRs work without modification
- CRs without `runtime` field auto-fallback to openclaw
- Observer refactoring is transparent to the OpenClaw adapter since `/workspace/` mount point is unified
