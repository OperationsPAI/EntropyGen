package handler

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	agentapi "github.com/entropyGen/entropyGen/internal/operator/api"

	"github.com/entropyGen/entropyGen/internal/backend/k8sclient"
)

// AgentHandler handles Agent CR CRUD operations.
type AgentHandler struct {
	client *k8sclient.AgentClient
}

func NewAgentHandler(client *k8sclient.AgentClient) *AgentHandler {
	return &AgentHandler{client: client}
}

func (h *AgentHandler) List(c *gin.Context) {
	agents, err := h.client.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("LIST_FAILED", err.Error(), ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": agents})
}

func (h *AgentHandler) Create(c *gin.Context) {
	var body struct {
		Name string            `json:"name" binding:"required"`
		Spec agentapi.AgentSpec `json:"spec" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, apiError("INVALID_REQUEST", err.Error(), ""))
		return
	}
	agent, err := h.client.Create(c.Request.Context(), body.Name, body.Spec)
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
	c.JSON(http.StatusOK, gin.H{"success": true, "data": agent})
}

func (h *AgentHandler) Update(c *gin.Context) {
	var spec agentapi.AgentSpec
	if err := c.ShouldBindJSON(&spec); err != nil {
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
