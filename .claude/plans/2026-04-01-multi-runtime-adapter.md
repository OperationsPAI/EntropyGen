# Multi-Runtime Agent Architecture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Decouple the platform from OpenClaw by introducing a pluggable Runtime Adapter abstraction, so any AI agent framework can be integrated as a black box.

**Architecture:** Replace OpenClaw-specific code in CRD, Operator, and Observer with a generic Runtime Contract (env vars + standard mount paths). Each agent framework becomes an independent Docker image (adapter) that satisfies the contract. Observability shifts from reading OpenClaw internals to monitoring `/workspace/` file changes + LiteLLM conversation logs.

**Tech Stack:** Go (Operator, Observer, Backend), TypeScript/React (Frontend), Docker, Kubernetes CRD, Helm

**Design doc:** [.claude/designs/multi-runtime.md](../designs/multi-runtime.md)

---

### Task 1: Update Agent CRD — Add `spec.runtime`, Remove `spec.cron`

**Files:**
- Modify: `k8s/crds/agent-crd.yaml`
- Modify: `k8s/helm/templates/crd.yaml` (if it duplicates the CRD)

- [ ] **Step 1: Add `runtime` object to CRD spec**

In `k8s/crds/agent-crd.yaml`, add `runtime` under `spec.properties`:

```yaml
                runtime:
                  type: object
                  properties:
                    type:
                      type: string
                      default: "openclaw"
                      description: "Runtime adapter type (e.g. openclaw, claude-code, aider)"
                    image:
                      type: string
                      description: "Override default image for this runtime type"
                    env:
                      type: array
                      items:
                        type: object
                        properties:
                          name:
                            type: string
                          value:
                            type: string
                      description: "Extra environment variables for the adapter"
```

- [ ] **Step 2: Remove `cron` from CRD spec**

Remove the `cron` object (lines 25-29 in `k8s/crds/agent-crd.yaml`):

```yaml
                # DELETE this block:
                cron:
                  type: object
                  properties:
                    schedule:
                      type: string
                      description: "Cron expression"
```

- [ ] **Step 3: Remove `runtimeImage` from CRD spec**

Remove the `runtimeImage` field (lines 87-89 in `k8s/crds/agent-crd.yaml`):

```yaml
                # DELETE this block:
                runtimeImage:
                  type: string
                  description: "Override the default agent-runtime container image"
```

- [ ] **Step 4: Add `Runtime` printer column**

Add a new printer column for runtime type:

```yaml
        - name: Runtime
          type: string
          jsonPath: .spec.runtime.type
```

- [ ] **Step 5: Verify Helm CRD template**

Check if `k8s/helm/templates/crd.yaml` duplicates the CRD definition. If so, apply the same changes. If it references `k8s/crds/agent-crd.yaml`, no action needed.

- [ ] **Step 6: Commit**

```bash
git add k8s/crds/agent-crd.yaml k8s/helm/templates/crd.yaml
git commit -m "feat: add spec.runtime to Agent CRD, remove spec.cron and spec.runtimeImage"
```

---

### Task 2: Update Go API Types

**Files:**
- Modify: `internal/operator/api/types.go`
- Modify: `internal/operator/api/zz_generated.deepcopy.go`

- [ ] **Step 1: Add `AgentRuntime` struct and update `AgentSpec`**

In `internal/operator/api/types.go`:

1. Add a new struct:

```go
type AgentRuntime struct {
	Type  string        `json:"type,omitempty"`
	Image string        `json:"image,omitempty"`
	Env   []AgentEnvVar `json:"env,omitempty"`
}

type AgentEnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
```

2. In `AgentSpec`, replace `RuntimeImage string` and `Cron *AgentCron` with:

```go
type AgentSpec struct {
	Role        string           `json:"role"`
	DisplayName string           `json:"displayName,omitempty"`
	Runtime     *AgentRuntime    `json:"runtime,omitempty"`     // NEW: replaces Cron + RuntimeImage
	LLM         *AgentLLM        `json:"llm,omitempty"`
	Gitea       *AgentGitea      `json:"gitea,omitempty"`
	Kubernetes  *AgentKubernetes `json:"kubernetes,omitempty"`
	Resources   *AgentResources  `json:"resources,omitempty"`
	Memory      *AgentMemory     `json:"memory,omitempty"`
	Paused      bool             `json:"paused,omitempty"`
}
```

3. Remove the `AgentCron` struct entirely.

- [ ] **Step 2: Update `zz_generated.deepcopy.go`**

Add DeepCopy methods for `AgentRuntime` and `AgentEnvVar`. Remove the `AgentCron` DeepCopy method:

```go
func (in *AgentRuntime) DeepCopyInto(out *AgentRuntime) {
	*out = *in
	if in.Env != nil {
		in, out := &in.Env, &out.Env
		*out = make([]AgentEnvVar, len(*in))
		copy(*out, *in)
	}
}

func (in *AgentRuntime) DeepCopy() *AgentRuntime {
	if in == nil {
		return nil
	}
	out := new(AgentRuntime)
	in.DeepCopyInto(out)
	return out
}
```

Update `AgentSpec.DeepCopyInto` to handle the new `Runtime` field instead of `Cron` and `RuntimeImage`.

- [ ] **Step 3: Build to verify compilation**

Run: `cd /home/ddq/AoyangSpace/EntropyGen && go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/operator/api/types.go internal/operator/api/zz_generated.deepcopy.go
git commit -m "feat: add AgentRuntime type, remove AgentCron and RuntimeImage from API types"
```

---

### Task 3: Refactor Operator — Generic ConfigMap + Image Selection

**Files:**
- Modify: `internal/operator/reconciler/k8s_resources.go`
- Modify: `internal/operator/reconciler/resource_reconciler.go`
- Modify: `internal/operator/reconciler/config_test.go`

- [ ] **Step 1: Add runtime image registry and selection function**

In `k8s_resources.go`, replace the existing `agentRuntimeImageName()` and `agentRuntimeImage()` functions with:

```go
// runtimeImages maps runtime type identifiers to default container images.
var runtimeImages = map[string]string{
	"openclaw":    "",  // populated from AGENT_RUNTIME_IMAGE env var
	"claude-code": "",  // future adapters will add their default images
	"aider":       "",
}

// agentRuntimeImage selects the container image based on spec.runtime.
// Priority: spec.runtime.image > registry lookup by type > env var fallback.
func agentRuntimeImage(agent *agentapi.Agent) string {
	// Explicit image override takes priority
	if agent.Spec.Runtime != nil && agent.Spec.Runtime.Image != "" {
		return agent.Spec.Runtime.Image
	}

	// Lookup by runtime type
	runtimeType := "openclaw" // default
	if agent.Spec.Runtime != nil && agent.Spec.Runtime.Type != "" {
		runtimeType = agent.Spec.Runtime.Type
	}

	if img, ok := runtimeImages[runtimeType]; ok && img != "" {
		return img
	}

	// Fallback to env var (backward compat)
	if v := os.Getenv("AGENT_RUNTIME_IMAGE"); v != "" {
		return v
	}
	return "registry.local/agent-runtime:latest"
}
```

- [ ] **Step 2: Replace `buildConfigMapData` — remove `openclaw.json` generation**

Replace the entire `buildConfigMapData` function with a version that only stores raw Role files (no openclaw.json):

```go
// buildPromptConfigMapData creates the data for agent-{name}-prompt ConfigMap.
// It stores raw Role files without framework-specific transformations.
func buildPromptConfigMapData(agent *agentapi.Agent, rd *roleData) map[string]string {
	result := map[string]string{}

	if rd != nil && rd.Soul != "" {
		result["SOUL.md"] = rd.Soul
	}
	if rd != nil && rd.AgentsMD != "" {
		result["AGENTS.md"] = rd.AgentsMD
	}
	if rd != nil && rd.Prompt != "" {
		prompt := rd.Prompt
		if agent.Spec.Gitea != nil && len(agent.Spec.Gitea.Repos) > 0 {
			prompt = strings.ReplaceAll(prompt, "{{REPOS}}", strings.Join(agent.Spec.Gitea.Repos, ","))
		}
		prompt = strings.ReplaceAll(prompt, "{{AGENT_ID}}", "agent-"+agent.Name)
		prompt = strings.ReplaceAll(prompt, "{{AGENT_ROLE}}", agent.Spec.Role)
		result["PROMPT.md"] = prompt
	}

	return result
}
```

- [ ] **Step 3: Update `EnsureConfigMap` to use the new function**

Rename the ConfigMap from `agent-{name}-config` to `agent-{name}-prompt` and use `buildPromptConfigMapData`:

```go
func (r *ResourceReconciler) EnsureConfigMap(ctx context.Context, agent *agentapi.Agent) error {
	data := buildPromptConfigMapData(agent, r.roleData)
	hash := computeHash(data)
	name := fmt.Sprintf("agent-%s-prompt", agent.Name)

	// ... rest of logic unchanged, just uses new name and data ...
}
```

- [ ] **Step 4: Update `buildDeployment` — generic env vars, new mount paths**

Replace the environment variables and volume mounts in `buildDeployment`:

```go
func buildDeployment(agent *agentapi.Agent, rd *roleData, cfgHash string, gatewayURL string, redisAddr string, giteaURL string, llmBaseURL string, llmAPIKey string) *appsv1.Deployment {
	// ... (replicas, names setup unchanged) ...

	promptCMName := fmt.Sprintf("agent-%s-prompt", agent.Name) // renamed from config
	// ... other names unchanged ...

	model := ""
	if agent.Spec.LLM != nil && agent.Spec.LLM.Model != "" {
		model = agent.Spec.LLM.Model
	}

	agentRepos := ""
	if agent.Spec.Gitea != nil && len(agent.Spec.Gitea.Repos) > 0 {
		agentRepos = strings.Join(agent.Spec.Gitea.Repos, ",")
	}

	// Generic environment variables (Runtime Contract §2.1)
	envVars := []corev1.EnvVar{
		{Name: "AGENT_ID", Value: "agent-" + agent.Name},
		{Name: "AGENT_ROLE", Value: agent.Spec.Role},
		{Name: "LLM_BASE_URL", Value: llmBaseURL},
		{Name: "LLM_API_KEY", Value: llmAPIKey},
		{Name: "LLM_MODEL", Value: model},
		{Name: "GITEA_BASE_URL", Value: giteaURL},
		{Name: "GITEA_TOKEN_FILE", Value: "/agent/secrets/gitea-token"},
		{Name: "GITEA_REPOS", Value: agentRepos},
		{Name: "WORKSPACE_DIR", Value: "/workspace"},
		{Name: "REDIS_ADDR", Value: redisAddr},
		{Name: "GATEWAY_URL", Value: gatewayURL},
	}

	// Append adapter-specific env vars from spec.runtime.env
	if agent.Spec.Runtime != nil {
		for _, e := range agent.Spec.Runtime.Env {
			envVars = append(envVars, corev1.EnvVar{Name: e.Name, Value: e.Value})
		}
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "prompt", MountPath: "/agent/prompt", ReadOnly: true},
		{Name: "skills", MountPath: "/agent/skills", ReadOnly: true},
		{Name: "jwt-token", MountPath: "/agent/secrets/jwt-token", SubPath: "token", ReadOnly: true},
		{Name: "gitea-token", MountPath: "/agent/secrets/gitea-token", SubPath: "token", ReadOnly: true},
		{Name: "workspace", MountPath: "/workspace"},
	}

	volumes := []corev1.Volume{
		{Name: "prompt", VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: promptCMName},
			},
		}},
		// ... skills, jwt-token, gitea-token, workspace unchanged except names ...
	}

	// ... rest of function ...
}
```

Note: `buildDeployment` now receives `llmBaseURL` and `llmAPIKey` as parameters since they're no longer baked into `openclaw.json`.

- [ ] **Step 5: Update `EnsureDeployment` to reference the renamed ConfigMap**

Change `agent-{name}-config` to `agent-{name}-prompt` in the hash lookup:

```go
func (r *ResourceReconciler) EnsureDeployment(ctx context.Context, agent *agentapi.Agent) error {
	cmName := fmt.Sprintf("agent-%s-prompt", agent.Name) // was agent-{name}-config
	// ... rest unchanged ...
	desired := buildDeployment(agent, r.roleData, cfgHash, r.GatewayURL, r.RedisAddr, r.GiteaURL, r.LLMBaseURL, r.LLMAPIKey)
	// ...
}
```

- [ ] **Step 6: Update `CronPrompt` in `resource_reconciler.go`**

The `CronPrompt` method stays the same since it only reads `roleData.Prompt`. No changes needed here — it's consumed by the cron scheduler which is outside this scope.

- [ ] **Step 7: Update tests in `config_test.go`**

Replace the `openclaw.json`-dependent tests with tests for `buildPromptConfigMapData`:

```go
func TestBuildPromptConfigMapData_RoleDataUsed(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Name = "dev-1"
	agent.Spec.Role = "developer"
	agent.Spec.LLM = &agentapi.AgentLLM{Model: "openai/gpt-4o"}

	rd := &roleData{
		Soul:     "role soul content",
		Prompt:   "role prompt for {{AGENT_ID}}",
		AgentsMD: "# Custom Agents\nCustom role agents.",
	}

	data := buildPromptConfigMapData(agent, rd)

	if data["SOUL.md"] != "role soul content" {
		t.Errorf("SOUL.md: got %q, want %q", data["SOUL.md"], "role soul content")
	}
	if data["AGENTS.md"] != "# Custom Agents\nCustom role agents." {
		t.Errorf("AGENTS.md: got %q", data["AGENTS.md"])
	}
	if data["PROMPT.md"] != "role prompt for agent-dev-1" {
		t.Errorf("PROMPT.md template substitution failed: got %q", data["PROMPT.md"])
	}
	// No openclaw.json should exist
	if _, exists := data["openclaw.json"]; exists {
		t.Error("openclaw.json should not be generated in generic mode")
	}
}

func TestBuildPromptConfigMapData_NilRoleData(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Spec.Role = "developer"

	data := buildPromptConfigMapData(agent, nil)

	if len(data) != 0 {
		t.Errorf("expected empty map without roleData, got %d entries", len(data))
	}
}

func TestAgentRuntimeImage_Selection(t *testing.T) {
	tests := []struct {
		name     string
		runtime  *agentapi.AgentRuntime
		envVar   string
		expected string
	}{
		{
			name:     "nil runtime defaults to env var",
			runtime:  nil,
			envVar:   "registry.local/custom:v1",
			expected: "registry.local/custom:v1",
		},
		{
			name:     "explicit image overrides type",
			runtime:  &agentapi.AgentRuntime{Type: "openclaw", Image: "my-custom:v2"},
			expected: "my-custom:v2",
		},
		{
			name:     "empty runtime defaults to openclaw",
			runtime:  &agentapi.AgentRuntime{},
			envVar:   "registry.local/default:v1",
			expected: "registry.local/default:v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVar != "" {
				t.Setenv("AGENT_RUNTIME_IMAGE", tt.envVar)
			}
			agent := &agentapi.Agent{}
			agent.Spec.Runtime = tt.runtime
			got := agentRuntimeImage(agent)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}
```

Remove the old `TestBuildConfigMapData_OpencalwModel`, `TestBuildConfigMapData_CustomModel`, `TestBuildConfigMapData_RoleDataUsed`, and `TestBuildConfigMapData_NilRoleData` tests.

Keep the `TestBuildSkillsData_*`, `TestComputeHash_*`, `TestReadRoleDataFromDir_*`, and `TestBuildSkillItems_*` tests unchanged.

- [ ] **Step 8: Run tests**

Run: `cd /home/ddq/AoyangSpace/EntropyGen && go test ./internal/operator/...`
Expected: All tests pass

- [ ] **Step 9: Build to verify compilation**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 10: Commit**

```bash
git add internal/operator/
git commit -m "feat: refactor Operator to generic Runtime Contract, remove openclaw.json generation"
```

---

### Task 4: Refactor Observer — Remove Session JSONL, Focus on Workspace

**Files:**
- Modify: `internal/observer/server.go`
- Modify: `internal/observer/watcher.go`
- Modify: `internal/observer/ws.go`
- Modify: `internal/observer/state.go`
- Delete: `internal/observer/session.go`
- Modify: `cmd/observer/main.go`

- [ ] **Step 1: Remove session-related routes from `server.go`**

Remove the session endpoints and update the router:

```go
func (s *Server) setupRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/healthz", s.handleHealthz)
	r.GET("/workspace/tree", s.handleWorkspaceTree)
	r.GET("/workspace/file", s.handleWorkspaceFile)
	r.GET("/workspace/diff", s.handleWorkspaceDiff)
	r.GET("/ws/live", s.handleWSLive)
	r.GET("/state", s.handleState)

	return r
}
```

Remove the `handleListSessions`, `handleCurrentSession`, `handleGetSession` methods and the `writeNDJSON` helper.

Update `Config` to remove `CompletionsDir` and `OpenClawHome`:

```go
type Config struct {
	Port         string
	WorkspaceDir string
}
```

Update `handleState` to read state from workspace:

```go
func (s *Server) handleState(c *gin.Context) {
	state := ReadState(s.cfg.WorkspaceDir)
	c.JSON(http.StatusOK, state)
}
```

- [ ] **Step 2: Simplify `watcher.go` — remove JSONL monitoring**

Remove `completionsDir` from `Watcher`, remove `JSONLLineEvent` type, remove JSONL-specific logic:

```go
type Watcher struct {
	workspaceDir string
	events       chan FileChangeEvent // simplified: only FileChangeEvent
}

func NewWatcher(workspaceDir string) *Watcher {
	return &Watcher{
		workspaceDir: workspaceDir,
		events:       make(chan FileChangeEvent, 256),
	}
}

func (w *Watcher) Events() <-chan FileChangeEvent {
	return w.events
}

func (w *Watcher) Run(ctx context.Context) error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer fsw.Close()

	if err := addDirRecursive(fsw, w.workspaceDir); err != nil {
		slog.Warn("watcher: cannot watch workspace dir", "dir", w.workspaceDir, "err", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-fsw.Events:
			if !ok {
				return nil
			}
			w.handleEvent(event)
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = fsw.Add(event.Name)
				}
			}
		case err, ok := <-fsw.Errors:
			if !ok {
				return nil
			}
			slog.Warn("watcher: fsnotify error", "err", err)
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	path := event.Name

	if !strings.HasPrefix(path, w.workspaceDir) {
		return
	}

	relPath, err := filepath.Rel(w.workspaceDir, path)
	if err != nil {
		return
	}

	// Skip hidden files/dirs
	for _, part := range strings.Split(relPath, string(filepath.Separator)) {
		if strings.HasPrefix(part, ".") {
			return
		}
	}

	var action string
	switch {
	case event.Has(fsnotify.Create):
		action = "created"
	case event.Has(fsnotify.Write):
		action = "modified"
	case event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename):
		action = "deleted"
	default:
		return
	}

	select {
	case w.events <- FileChangeEvent{Path: relPath, Action: action}:
	default:
	}
}
```

- [ ] **Step 3: Simplify `ws.go` — remove JSONL broadcast**

Update `broadcastEvent` to only handle `FileChangeEvent`:

```go
func (h *WSHub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-h.watcher.Events():
			if !ok {
				return
			}
			h.broadcastEvent(evt)
		}
	}
}

func (h *WSHub) broadcastEvent(evt FileChangeEvent) {
	msg, err := json.Marshal(map[string]interface{}{
		"type":   "file_change",
		"path":   evt.Path,
		"action": evt.Action,
	})
	if err != nil {
		slog.Warn("ws: marshal error", "err", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- msg:
		default:
		}
	}
}
```

Remove the `readLastLine` function.

- [ ] **Step 4: Update `state.go` — read from workspace dir**

Change `ReadState` to read from the workspace directory instead of OpenClaw home:

```go
func ReadState(workspaceDir string) AgentState {
	var state AgentState
	data, err := os.ReadFile(filepath.Join(workspaceDir, "state.json"))
	if err != nil {
		return state
	}
	_ = json.Unmarshal(data, &state)
	return state
}
```

- [ ] **Step 5: Delete `session.go`**

Remove `internal/observer/session.go` entirely — all session-related functionality is removed.

- [ ] **Step 6: Update `cmd/observer/main.go`**

Remove `completionsDir` and `openClawHome` references. Use only `workspaceDir`:

```go
func main() {
	port := envOr("OBSERVER_PORT", "8081")
	workspaceDir := envOr("WORKSPACE_DIR", "/workspace")

	ensureDir(workspaceDir)

	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	cfg := observer.Config{
		Port:         port,
		WorkspaceDir: workspaceDir,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	watcher := observer.NewWatcher(workspaceDir)
	wsHub := observer.NewWSHub(watcher)

	go func() {
		if err := watcher.Run(ctx); err != nil && ctx.Err() == nil {
			slog.Error("watcher failed", "err", err)
		}
	}()
	go wsHub.Run(ctx)

	// Start poller if Redis is configured
	redisAddr := os.Getenv("REDIS_ADDR")
	agentID := os.Getenv("AGENT_ID")
	openclawToken := os.Getenv("OPENCLAW_GATEWAY_TOKEN")
	if redisAddr != "" && agentID != "" {
		rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
		stream := fmt.Sprintf("events:%s", agentID)
		reader := redisclient.NewStreamReader(rdb, "observer", "poller-0")
		openclawURL := envOr("OPENCLAW_URL", "http://127.0.0.1:8080")
		poller := observer.NewPoller(reader, stream, openclawURL, openclawToken)
		go func() {
			if err := poller.Run(ctx); err != nil && ctx.Err() == nil {
				slog.Error("poller failed", "err", err)
			}
		}()
		slog.Info("poller enabled", "stream", stream)
	}

	srv := observer.NewServer(cfg, wsHub)
	slog.Info("observer starting", "addr", ":"+port, "workspace", workspaceDir)
	if err := srv.Run(); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 7: Update observer tests**

Run: `go test ./internal/observer/...`

Fix any compilation errors caused by removed types (`JSONLLineEvent`, `CompletionsDir`, etc.). The existing `observer_test.go` may reference removed functions — update or remove affected tests.

- [ ] **Step 8: Build to verify compilation**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 9: Commit**

```bash
git add internal/observer/ cmd/observer/
git rm internal/observer/session.go
git commit -m "feat: refactor Observer to workspace-only monitoring, remove OpenClaw session dependencies"
```

---

### Task 5: Restructure agent-runtime Directory

**Files:**
- Create: `agent-runtimes/common/setup-git.sh`
- Create: `agent-runtimes/openclaw/Dockerfile`
- Create: `agent-runtimes/openclaw/entrypoint.sh`
- Modify: `skaffold.yaml` (update Dockerfile path)

- [ ] **Step 1: Create common setup script**

Create `agent-runtimes/common/setup-git.sh`:

```bash
#!/bin/bash
# Common git setup for all runtime adapters.
# Sources: AGENT_ID, GITEA_BASE_URL, GITEA_TOKEN_FILE env vars.
set -euo pipefail

git config --global user.name "${AGENT_ID:-agent}"
git config --global user.email "${AGENT_ID:-agent}@platform.local"

if [ -f "${GITEA_TOKEN_FILE:-/agent/secrets/gitea-token}" ]; then
    TOKEN=$(cat "${GITEA_TOKEN_FILE}")
    GITEA_URL="${GITEA_BASE_URL:-http://gitea.aidevops.svc:3000}"
    GITEA_URL=$(echo "$GITEA_URL" | sed 's|/api/v1$||')
    CRED_URL=$(echo "$GITEA_URL" | sed "s|://|://${AGENT_ID:-agent}:${TOKEN}@|")
    echo "$CRED_URL" > ~/.git-credentials
    chmod 600 ~/.git-credentials
    git config --global credential.helper store
fi
```

- [ ] **Step 2: Create OpenClaw adapter Dockerfile**

Create `agent-runtimes/openclaw/Dockerfile`:

```dockerfile
# Stage 1: Build gitea-cli
FROM golang:1.25-alpine AS gitea-cli-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/gitea-cli/ cmd/gitea-cli/
COPY internal/gitea-cli/ internal/gitea-cli/
RUN CGO_ENABLED=0 GOOS=linux go build -o gitea ./cmd/gitea-cli/

# Stage 2: Build observer
FROM golang:1.25-alpine AS observer-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/observer/ cmd/observer/
COPY internal/observer/ internal/observer/
COPY internal/common/ internal/common/
RUN CGO_ENABLED=0 GOOS=linux go build -o agent-observer ./cmd/observer

# Stage 3: Agent Runtime image
FROM debian:bookworm-slim

USER root

RUN sed -i 's/deb.debian.org/mirrors.ustc.edu.cn/g' /etc/apt/sources.list.d/debian.sources \
    && apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://dl.k8s.io/release/$(curl -fsSL https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
    -o /usr/local/bin/kubectl \
    && chmod +x /usr/local/bin/kubectl

RUN curl -fsSL https://openclaw.ai/install.sh | bash -s -- --no-prompt --no-onboard

COPY --from=gitea-cli-builder /app/gitea /usr/local/bin/gitea
COPY --from=observer-builder /app/agent-observer /usr/local/bin/agent-observer

# Common setup script
COPY agent-runtimes/common/setup-git.sh /common/setup-git.sh
RUN chmod +x /common/setup-git.sh

# Entrypoint
COPY agent-runtimes/openclaw/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

RUN useradd -m -u 1000 -s /bin/bash node \
    && mkdir -p /home/node/.openclaw \
    && chown -R node:node /home/node/.openclaw
USER node

COPY --chown=node:node agent-runtime/skills/ /home/node/.openclaw/skills/
COPY --chown=node:node agent-runtime/builtin-role/workspace-templates/ /agent/workspace-templates/

ENTRYPOINT ["/entrypoint.sh"]
```

- [ ] **Step 3: Create OpenClaw adapter entrypoint**

Create `agent-runtimes/openclaw/entrypoint.sh`:

```bash
#!/bin/bash
set -euo pipefail

# ---- Common layer ----
source /common/setup-git.sh

# ---- Adapter layer: OpenClaw-specific ----

# Copy prompt files to openclaw home
[ -d /agent/prompt ] && cp -f /agent/prompt/* ~/.openclaw/ 2>/dev/null || true

# Copy skills
[ -d /agent/skills ] && mkdir -p ~/.openclaw/skills && cp -rf /agent/skills/* ~/.openclaw/skills/ 2>/dev/null || true

# Copy role-specific extra files
[ -d /agent/role ] && cp -f /agent/role/* ~/.openclaw/ 2>/dev/null || true

# Override openclaw workspace files with platform templates
if [ -d /agent/workspace-templates ]; then
    mkdir -p ~/.openclaw/workspace
    for f in /agent/workspace-templates/*; do
        [ -f "$f" ] && cp -f "$f" ~/.openclaw/workspace/
    done
    for f in ~/.openclaw/workspace/*.md; do
        [ -f "$f" ] && sed -i \
            "s|{{AGENT_ID}}|${AGENT_ID:-unknown}|g; \
             s|{{AGENT_ROLE}}|${AGENT_ROLE:-agent}|g; \
             s|{{REPOS}}|${GITEA_REPOS:-}|g; \
             s|{{GITEA_URL}}|${GITEA_BASE_URL:-http://gitea.aidevops.svc:3000}|g" \
            "$f"
    done
    rm -f ~/.openclaw/workspace/BOOTSTRAP.md
fi

# Template variable substitution in prompt files
[ -f ~/.openclaw/SOUL.md ] && sed -i "s|{{AGENT_ID}}|${AGENT_ID:-unknown}|g; s|{{AGENT_ROLE}}|${AGENT_ROLE:-agent}|g" ~/.openclaw/SOUL.md
[ -f ~/.openclaw/PROMPT.md ] && sed -i \
    "s|{{AGENT_ID}}|${AGENT_ID:-unknown}|g; \
     s|{{AGENT_ROLE}}|${AGENT_ROLE:-agent}|g; \
     s|{{REPOS}}|${GITEA_REPOS:-}|g; \
     s|{{GITEA_URL}}|${GITEA_BASE_URL:-http://gitea.aidevops.svc:3000}|g" \
    ~/.openclaw/PROMPT.md

# Generate openclaw.json from generic env vars
LLM_PROVIDER="litellm"
LLM_MODEL_ID="${LLM_MODEL#*/}"
if [ "$LLM_MODEL_ID" = "$LLM_MODEL" ]; then
    LLM_MODEL="litellm/${LLM_MODEL}"
fi

cat > ~/.openclaw/openclaw.json <<EOCFG
{
  "models": {
    "providers": {
      "${LLM_PROVIDER}": {
        "baseUrl": "${LLM_BASE_URL:-http://litellm.llm-gateway.svc:4000/v1}",
        "apiKey": "${LLM_API_KEY:-sk-placeholder}",
        "api": "openai-completions",
        "models": [{
          "id": "${LLM_MODEL_ID}",
          "name": "${LLM_MODEL_ID}",
          "reasoning": false,
          "input": ["text"],
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
  },
  "gateway": {
    "controlUi": {
      "dangerouslyAllowHostHeaderOriginFallback": true
    },
    "http": {
      "endpoints": {
        "chatCompletions": { "enabled": true }
      }
    }
  }
}
EOCFG

# Generate gateway token
OPENCLAW_GATEWAY_TOKEN="${OPENCLAW_GATEWAY_TOKEN:-$(head -c 16 /dev/urandom | base64)}"
export OPENCLAW_GATEWAY_TOKEN

# Start observer in background
agent-observer &

# Start openclaw gateway
exec openclaw gateway run --port 8080 --bind lan --allow-unconfigured --token "$OPENCLAW_GATEWAY_TOKEN"
```

- [ ] **Step 4: Update `skaffold.yaml` — point to new Dockerfile path**

In `skaffold.yaml`, update the agent-runtime artifact to use the new path:

```yaml
  - image: 10.10.10.240/library/agent-runtime
    docker:
      dockerfile: agent-runtimes/openclaw/Dockerfile
    context: .
```

Apply the same change in the minikube profile.

- [ ] **Step 5: Verify build**

Run: `docker build -f agent-runtimes/openclaw/Dockerfile -t test-openclaw .` (or use skaffold)
Expected: Image builds successfully

- [ ] **Step 6: Commit**

```bash
git add agent-runtimes/
git add skaffold.yaml
git commit -m "feat: restructure agent-runtime into agent-runtimes/ with common layer + openclaw adapter"
```

---

### Task 6: Update Backend API — Runtime Types

**Files:**
- Modify: `internal/backend/handler/agents.go`
- Modify: `frontend/src/types/agent.ts`
- Modify: `frontend/src/types/observe.ts`

- [ ] **Step 1: Update `RuntimeImages` handler to return runtime types**

In `internal/backend/handler/agents.go`, replace the `RuntimeImages` handler to return runtime type info:

```go
// RuntimeTypes returns the available runtime adapter types and their default images.
func (h *AgentHandler) RuntimeTypes(c *gin.Context) {
	type runtimeEntry struct {
		Type    string `json:"type"`
		Image   string `json:"image"`
		Default bool   `json:"default"`
	}

	defaultImg := os.Getenv("AGENT_RUNTIME_IMAGE")
	if defaultImg == "" {
		defaultImg = "registry.local/agent-runtime:latest"
	}

	runtimes := []runtimeEntry{
		{Type: "openclaw", Image: defaultImg, Default: true},
	}

	// Additional runtime types from AGENT_RUNTIME_TYPES env (format: type=image,type=image)
	if extra := os.Getenv("AGENT_RUNTIME_TYPES"); extra != "" {
		for _, entry := range strings.Split(extra, ",") {
			parts := strings.SplitN(strings.TrimSpace(entry), "=", 2)
			if len(parts) == 2 {
				runtimes = append(runtimes, runtimeEntry{Type: parts[0], Image: parts[1]})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": runtimes})
}
```

- [ ] **Step 2: Update API route**

In `internal/backend/api/router.go`, rename the route:

```go
opt.GET("/agents/runtime-types", handler.RequireRole("member", "admin"), agentH.RuntimeTypes)
```

- [ ] **Step 3: Update frontend TypeScript types**

In `frontend/src/types/agent.ts`, replace `runtimeImage` with `runtime`:

```typescript
export interface RuntimeConfig {
  type: string
  image?: string
  env?: Array<{ name: string; value: string }>
}

export interface AgentSpec {
  role: string
  llm: LLMConfig
  runtime?: RuntimeConfig   // NEW: replaces runtimeImage
  resources: ResourceConfig
  gitea: GiteaConfig
}
```

Remove `CronConfig` interface and `cron` from `AgentSpec`.

- [ ] **Step 4: Update frontend observe types**

In `frontend/src/types/observe.ts`, remove session-related types:

Remove: `SessionInfo`, `SessionMessage`, `ModelChangeMessage`, `ThinkingLevelChangeMessage`, `CustomMessage`, `MessageEnvelope`, `JsonlMessage`, `JsonlLiveEvent`, and all related types.

Keep: `FileTreeNode`, `FileContentResponse`, `DiffResultResponse`, `FileChangeEvent`.

Simplify `SidecarWsEvent`:

```typescript
export type SidecarWsEvent = FileChangeEvent
```

- [ ] **Step 5: Build frontend to verify**

Run: `cd frontend && npm run build` (or `tsc --noEmit`)
Expected: No type errors (there will likely be cascading errors in Observe pages — these are addressed in Task 7)

- [ ] **Step 6: Commit**

```bash
git add internal/backend/ frontend/src/types/
git commit -m "feat: update Backend API and frontend types for runtime adapter abstraction"
```

---

### Task 7: Update Frontend Observe Page — Workspace-Only View

**Files:**
- Modify: `frontend/src/pages/Observe/ObserveDetail.tsx`
- Modify: `frontend/src/pages/Observe/ConversationFlow.tsx`
- Modify: Other Observe components as needed

This task adapts the frontend observe page to work without session JSONL data. The conversation panel should show a placeholder message directing users to check LiteLLM logs, and the workspace/file explorer panel continues working unchanged.

- [ ] **Step 1: Update `ObserveDetail.tsx`**

Remove session fetching logic (`/sessions`, `/sessions/current`). Keep workspace tree, file viewer, and diff panels. Replace conversation panel content with a note:

```tsx
// In the conversation panel area, replace session content with:
<div className={styles.placeholder}>
  <p>Conversation logs are available via LiteLLM dashboard.</p>
  <p>Workspace changes are shown in real-time below.</p>
</div>
```

- [ ] **Step 2: Simplify `ConversationFlow.tsx`**

If this component is solely for rendering JSONL session messages, it can be simplified to show workspace activity events from the WebSocket instead. Keep the component but repurpose it to show a feed of `file_change` events:

```tsx
// Show recent file changes as a simple activity log
interface ActivityEntry {
  path: string
  action: 'created' | 'modified' | 'deleted'
  timestamp: string
}
```

- [ ] **Step 3: Update WebSocket handler**

Remove `jsonl` event handling from the WebSocket connection in `ObserveDetail.tsx`. Only handle `file_change` events.

- [ ] **Step 4: Verify frontend builds**

Run: `cd frontend && npm run build`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/Observe/
git commit -m "feat: update Observe page to workspace-only view, remove session JSONL dependency"
```

---

### Task 8: Update Frontend Agent Creation — Runtime Selector

**Files:**
- Modify: `frontend/src/pages/Agents/NewAgent.tsx`
- Modify: `frontend/src/pages/Agents/Detail.tsx`
- Modify: `frontend/src/api/agents.ts`

- [ ] **Step 1: Update `NewAgent.tsx` — add runtime type selector**

Replace the `runtimeImage` field (if present) with a runtime type dropdown:

```tsx
<FormControl>
  <FormLabel>Runtime Type</FormLabel>
  <Select
    value={spec.runtime?.type || 'openclaw'}
    onChange={(e) => setSpec({
      ...spec,
      runtime: { ...spec.runtime, type: e.target.value }
    })}
  >
    {runtimeTypes.map(rt => (
      <option key={rt.type} value={rt.type}>{rt.type}</option>
    ))}
  </Select>
</FormControl>
```

Fetch runtime types from the new API endpoint:

```typescript
const [runtimeTypes, setRuntimeTypes] = useState<RuntimeType[]>([])

useEffect(() => {
  fetch('/api/agents/runtime-types')
    .then(r => r.json())
    .then(data => setRuntimeTypes(data.data))
}, [])
```

- [ ] **Step 2: Update `Detail.tsx` — show runtime type**

Add runtime type display in the agent detail view.

- [ ] **Step 3: Update `agents.ts` API client**

Update the API endpoint from `runtime-images` to `runtime-types` and adjust the response type.

- [ ] **Step 4: Remove cron schedule from agent creation form**

Remove any cron schedule input fields from the new agent form.

- [ ] **Step 5: Verify frontend builds**

Run: `cd frontend && npm run build`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add frontend/src/pages/Agents/ frontend/src/api/
git commit -m "feat: add runtime type selector to agent creation, remove cron fields"
```

---

### Task 9: Clean Up Old agent-runtime Directory

**Files:**
- Delete: `agent-runtime/Dockerfile`
- Delete: `agent-runtime/entrypoint.sh`
- Keep: `agent-runtime/skills/` and `agent-runtime/builtin-role/` (still referenced by new Dockerfile)

- [ ] **Step 1: Remove old Dockerfile and entrypoint**

```bash
git rm agent-runtime/Dockerfile agent-runtime/entrypoint.sh
```

- [ ] **Step 2: Move skills and builtin-role to new structure**

Move the skills and workspace templates into the openclaw adapter directory since they're OpenClaw-specific:

```bash
mv agent-runtime/skills/ agent-runtimes/openclaw/skills/
mv agent-runtime/builtin-role/ agent-runtimes/openclaw/builtin-role/
```

Update `agent-runtimes/openclaw/Dockerfile` to use the new paths:

```dockerfile
COPY --chown=node:node agent-runtimes/openclaw/skills/ /home/node/.openclaw/skills/
COPY --chown=node:node agent-runtimes/openclaw/builtin-role/workspace-templates/ /agent/workspace-templates/
```

- [ ] **Step 3: Remove empty agent-runtime directory**

```bash
rmdir agent-runtime/ 2>/dev/null || true
```

- [ ] **Step 4: Verify build**

Run: `go build ./...` and verify skaffold config still works.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "chore: remove old agent-runtime/, move assets to agent-runtimes/openclaw/"
```

---

### Task 10: Integration Verification

- [ ] **Step 1: Run all Go tests**

```bash
cd /home/ddq/AoyangSpace/EntropyGen
go test ./...
```

Expected: All tests pass

- [ ] **Step 2: Run frontend build**

```bash
cd frontend && npm run build
```

Expected: No errors

- [ ] **Step 3: Verify Docker image builds**

```bash
docker build -f agent-runtimes/openclaw/Dockerfile -t test-agent-runtime .
```

Expected: Image builds successfully

- [ ] **Step 4: Verify CRD backward compatibility**

Create a test Agent CR with no `runtime` field and verify the Operator defaults to openclaw:

```yaml
apiVersion: aidevops.io/v1alpha1
kind: Agent
metadata:
  name: test-compat
spec:
  role: developer
  llm:
    model: "openai/gpt-4o"
```

Expected: Operator should treat this as `runtime.type: openclaw`

- [ ] **Step 5: Commit any remaining fixes**

```bash
git add -A
git commit -m "test: integration verification for multi-runtime refactor"
```
