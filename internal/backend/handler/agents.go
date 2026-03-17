package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	agentapi "github.com/entropyGen/entropyGen/internal/operator/api"

	"github.com/entropyGen/entropyGen/internal/backend/k8sclient"
	"github.com/entropyGen/entropyGen/internal/common/chclient"
	"github.com/entropyGen/entropyGen/internal/common/giteaclient"
	"github.com/entropyGen/entropyGen/internal/common/models"
	"github.com/entropyGen/entropyGen/internal/common/redisclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AgentHandler handles Agent CR CRUD operations.
type AgentHandler struct {
	client       *k8sclient.AgentClient
	giteaClient  *giteaclient.Client
	streamWriter *redisclient.StreamWriter
	ch           *chclient.Client
	namespace    string
	httpClient   *http.Client
}

func NewAgentHandler(client *k8sclient.AgentClient, giteaClient *giteaclient.Client, streamWriter *redisclient.StreamWriter, namespace string) *AgentHandler {
	return &AgentHandler{
		client:       client,
		giteaClient:  giteaClient,
		streamWriter: streamWriter,
		namespace:    namespace,
		httpClient: &http.Client{
			Timeout: 1 * time.Second,
		},
	}
}

// SetClickHouse sets the ClickHouse client for enriching agent data with audit metrics.
func (h *AgentHandler) SetClickHouse(ch *chclient.Client) {
	h.ch = ch
}

func (h *AgentHandler) List(c *gin.Context) {
	agents, err := h.client.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("LIST_FAILED", err.Error(), ""))
		return
	}
	// Enrich with ClickHouse audit data (token usage, last action)
	h.enrichAgents(c, agents)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": agents})
}

func (h *AgentHandler) Create(c *gin.Context) {
	var body struct {
		Name string          `json:"name" binding:"required"`
		Spec json.RawMessage `json:"spec" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, apiError("INVALID_REQUEST", err.Error(), ""))
		return
	}

	spec, err := unmarshalAgentSpec(body.Spec)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError("INVALID_SPEC", err.Error(), ""))
		return
	}

	agent, err := h.client.Create(c.Request.Context(), body.Name, spec)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("CREATE_FAILED", err.Error(), ""))
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": agent})
}

func (h *AgentHandler) Get(c *gin.Context) {
	agent, err := h.client.Get(c.Request.Context(), c.Param("name"))
	if err != nil {
		c.JSON(http.StatusNotFound,
			apiError("AGENT_NOT_FOUND", "agent not found", "agent '"+c.Param("name")+"' not found"))
		return
	}
	agents := []agentapi.Agent{*agent}
	h.enrichAgents(c, agents)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": agents[0]})
}

func (h *AgentHandler) Update(c *gin.Context) {
	raw, _ := io.ReadAll(c.Request.Body)
	spec, err := unmarshalAgentSpec(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError("INVALID_REQUEST", err.Error(), ""))
		return
	}
	agent, err := h.client.Update(c.Request.Context(), c.Param("name"), spec)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("UPDATE_FAILED", err.Error(), ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": agent})
}

func (h *AgentHandler) Delete(c *gin.Context) {
	if err := h.client.Delete(c.Request.Context(), c.Param("name")); err != nil {
		c.JSON(http.StatusInternalServerError, apiError("DELETE_FAILED", err.Error(), ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AgentHandler) Pause(c *gin.Context) {
	agent, err := h.client.SetPaused(c.Request.Context(), c.Param("name"), true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("PAUSE_FAILED", err.Error(), ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": agent})
}

func (h *AgentHandler) Resume(c *gin.Context) {
	agent, err := h.client.SetPaused(c.Request.Context(), c.Param("name"), false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("RESUME_FAILED", err.Error(), ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": agent})
}

func (h *AgentHandler) Logs(c *gin.Context) {
	logs, err := h.client.GetLogs(c.Request.Context(), c.Param("name"), 200) //nolint:mnd
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("LOGS_FAILED", err.Error(), ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": logs})
}

// ResetMemory clears the agent's workspace PVC and restarts the pod.
// This is a high-risk operation that destroys all agent memory/state.
func (h *AgentHandler) ResetMemory(c *gin.Context) {
	name := c.Param("name")
	if err := h.client.ResetMemory(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError,
			apiError("RESET_MEMORY_FAILED", "failed to reset agent memory", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RuntimeImages returns the list of available agent runtime images.
// The default image comes from AGENT_RUNTIME_IMAGE env var.
// Extra images can be added via AGENT_RUNTIME_IMAGES (comma-separated).
func (h *AgentHandler) RuntimeImages(c *gin.Context) {
	type imageEntry struct {
		Image   string `json:"image"`
		Default bool   `json:"default"`
	}

	defaultImg := os.Getenv("AGENT_RUNTIME_IMAGE")
	if defaultImg == "" {
		defaultImg = "registry.local/agent-runtime:latest"
	}

	images := []imageEntry{{Image: defaultImg, Default: true}}

	if extra := os.Getenv("AGENT_RUNTIME_IMAGES"); extra != "" {
		for _, img := range strings.Split(extra, ",") {
			img = strings.TrimSpace(img)
			if img != "" && img != defaultImg {
				images = append(images, imageEntry{Image: img})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": images})
}

// AssignIssue creates a Gitea issue and assigns it to the agent's Gitea user.
// It then writes an issue.assigned_by_admin event to the events:gitea Redis stream.
func (h *AgentHandler) AssignIssue(c *gin.Context) {
	if h.giteaClient == nil {
		c.JSON(http.StatusServiceUnavailable,
			apiError("GITEA_UNAVAILABLE", "gitea client not configured", ""))
		return
	}

	name := c.Param("name")
	ctx := c.Request.Context()

	var req struct {
		Repo     string   `json:"repo" binding:"required"`
		Title    string   `json:"title" binding:"required"`
		Body     string   `json:"body" binding:"required"`
		Labels   []string `json:"labels"`
		Priority string   `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError("INVALID_REQUEST", err.Error(), ""))
		return
	}

	// Look up the Agent CR to find the Gitea username.
	agent, err := h.client.Get(ctx, name)
	if err != nil {
		c.JSON(http.StatusNotFound,
			apiError("AGENT_NOT_FOUND", "agent not found", fmt.Sprintf("agent %q not found", name)))
		return
	}

	giteaUsername := resolveGiteaUsername(agent)
	if giteaUsername == "" {
		c.JSON(http.StatusBadRequest,
			apiError("NO_GITEA_USER", "agent has no gitea user configured",
				fmt.Sprintf("agent %q has no gitea username in spec or status", name)))
		return
	}

	// Parse owner/repo.
	parts := strings.SplitN(req.Repo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		c.JSON(http.StatusBadRequest,
			apiError("INVALID_REPO", "repo must be in owner/repo format", ""))
		return
	}

	// Create the issue and assign to the agent.
	result, err := h.giteaClient.CreateIssue(ctx, parts[0], parts[1], giteaclient.CreateIssueOpts{
		Title:    req.Title,
		Body:     req.Body,
		Labels:   req.Labels,
		Assignee: giteaUsername,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway,
			apiError("GITEA_CREATE_ISSUE_FAILED", "failed to create issue in gitea", err.Error()))
		return
	}

	// Write event to Redis stream.
	if h.streamWriter != nil {
		payload, _ := json.Marshal(models.IssueAssignedByAdminPayload{
			Repo:        req.Repo,
			IssueNumber: result.Number,
			IssueURL:    result.HTMLURL,
			Title:       req.Title,
			Labels:      req.Labels,
			Priority:    req.Priority,
			Assignee:    giteaUsername,
		})

		event := &models.Event{
			EventID:   uuid.New().String(),
			EventType: models.EventTypeIssueAssignedByAdmin,
			Timestamp: time.Now().UTC(),
			AgentID:   name,
			AgentRole: agent.Spec.Role,
			Payload:   payload,
		}

		// Fire-and-forget: log but don't fail the request if event write fails.
		if err := h.streamWriter.Write(ctx, "events:gitea", event, redisclient.MaxLenGitea); err != nil {
			c.Error(fmt.Errorf("write issue.assigned_by_admin event: %w", err)) //nolint:errcheck
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"issue_number": result.Number,
			"issue_url":    result.HTMLURL,
		},
	})
}

// resolveGiteaUsername returns the Gitea username for an agent,
// preferring status.giteaUser.username (actual provisioned name) over spec.gitea.username.
// unmarshalAgentSpec converts the frontend DTO format into the CRD AgentSpec.
// It handles the structural mismatch between the flat frontend fields and the
// nested CRD types (resources, gitea).
func unmarshalAgentSpec(raw json.RawMessage) (agentapi.AgentSpec, error) {
	// First try direct CRD format (e.g. from kubectl or raw API calls).
	var spec agentapi.AgentSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return spec, fmt.Errorf("invalid spec: %w", err)
	}

	// Parse frontend-specific flat fields that don't match CRD structure.
	var flat struct {
		Resources *struct {
			CPURequest    string `json:"cpuRequest"`
			CPULimit      string `json:"cpuLimit"`
			MemoryRequest string `json:"memoryRequest"`
			MemoryLimit   string `json:"memoryLimit"`
			WorkspaceSize string `json:"workspaceSize"`
		} `json:"resources"`
		Gitea *struct {
			Repo        string   `json:"repo"`
			Repos       []string `json:"repos"`
			Username    string   `json:"username"`
			Email       string   `json:"email"`
			Permissions []string `json:"permissions"`
		} `json:"gitea"`
	}
	_ = json.Unmarshal(raw, &flat)

	// Map flat resources → nested CRD format.
	if flat.Resources != nil && flat.Resources.CPURequest != "" {
		spec.Resources = &agentapi.AgentResources{
			Requests: &agentapi.ResourceRequirements{
				CPU:    flat.Resources.CPURequest,
				Memory: flat.Resources.MemoryRequest,
			},
			Limits: &agentapi.ResourceRequirements{
				CPU:    flat.Resources.CPULimit,
				Memory: flat.Resources.MemoryLimit,
			},
		}
		if flat.Resources.WorkspaceSize != "" {
			if spec.Memory == nil {
				spec.Memory = &agentapi.AgentMemory{}
			}
			spec.Memory.StorageSize = flat.Resources.WorkspaceSize
		}
	}

	// Map flat gitea → CRD format (preserve permissions, add repo context).
	if flat.Gitea != nil {
		if spec.Gitea == nil {
			spec.Gitea = &agentapi.AgentGitea{}
		}
		if flat.Gitea.Username != "" {
			spec.Gitea.Username = flat.Gitea.Username
		}
		if flat.Gitea.Email != "" {
			spec.Gitea.Email = flat.Gitea.Email
		}
		if len(flat.Gitea.Permissions) > 0 {
			spec.Gitea.Permissions = flat.Gitea.Permissions
		}
		if flat.Gitea.Repo != "" {
			spec.Gitea.Repos = []string{flat.Gitea.Repo}
		}
		if len(flat.Gitea.Repos) > 0 {
			spec.Gitea.Repos = flat.Gitea.Repos
		}
	}

	return spec, nil
}

func resolveGiteaUsername(agent *agentapi.Agent) string {
	if agent.Status.GiteaUser != nil && agent.Status.GiteaUser.Username != "" {
		return agent.Status.GiteaUser.Username
	}
	// Default convention: role-level Gitea user.
	if agent.Spec.Role != "" {
		return "role-" + agent.Spec.Role
	}
	return ""
}

// enrichAgents populates tokenUsage, lastAction, and currentTask from
// ClickHouse audit data and observer state.
// This is best-effort: if any source is unavailable, agents are returned without that enrichment.
func (h *AgentHandler) enrichAgents(c *gin.Context, agents []agentapi.Agent) {
	if len(agents) == 0 {
		return
	}

	// Enrich from ClickHouse (token usage, last action).
	if h.ch != nil {
		summaries, err := h.ch.GetAgentSummaries(c.Request.Context())
		if err == nil && len(summaries) > 0 {
			for i := range agents {
				agentID := "agent-" + agents[i].Name
				s, ok := summaries[agentID]
				if !ok {
					continue
				}
				agents[i].Status.TokenUsage = &agentapi.AgentTokenUsage{
					Today: int64(s.TodayTokens),
					Total: int64(s.TotalTokens),
				}
				if s.LastDescription != "" {
					agents[i].Status.LastAction = &agentapi.AgentLastAction{
						Description: s.LastDescription,
						Timestamp:   metav1.NewTime(s.LastTimestamp),
					}
				}
			}
		}
	}

	// Enrich from observer /state (currentTask).
	h.enrichCurrentTasks(agents)
}

// observerStateResponse mirrors the JSON returned by the observer /state endpoint.
type observerStateResponse struct {
	CurrentTask *struct {
		Type  string `json:"type"`
		ID    int    `json:"id"`
		Title string `json:"title"`
		Repo  string `json:"repo"`
	} `json:"current_task"`
}

// enrichCurrentTasks fetches /state from each agent's observer in parallel
// and populates CurrentTask on the agent status. Best-effort: failures are logged and skipped.
func (h *AgentHandler) enrichCurrentTasks(agents []agentapi.Agent) {
	var wg sync.WaitGroup
	for i := range agents {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			addr := SidecarAddr(agents[idx].Name, h.namespace)
			stateURL := fmt.Sprintf("http://%s/state", addr)

			resp, err := h.httpClient.Get(stateURL)
			if err != nil {
				slog.Debug("enrichCurrentTasks: failed to fetch state", "agent", agents[idx].Name, "err", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return
			}

			var state observerStateResponse
			if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
				slog.Debug("enrichCurrentTasks: failed to decode state", "agent", agents[idx].Name, "err", err)
				return
			}

			if state.CurrentTask != nil {
				agents[idx].Status.CurrentTask = &agentapi.CurrentTask{
					Type:   state.CurrentTask.Type,
					Number: state.CurrentTask.ID,
					Title:  state.CurrentTask.Title,
					Repo:   state.CurrentTask.Repo,
				}
			}
		}(i)
	}
	wg.Wait()
}
