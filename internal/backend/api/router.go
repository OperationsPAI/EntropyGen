package api

import (
	"github.com/gin-gonic/gin"

	"github.com/entropyGen/entropyGen/internal/backend/handler"
	"github.com/entropyGen/entropyGen/internal/backend/k8sclient"
	"github.com/entropyGen/entropyGen/internal/backend/wspush"
	"github.com/entropyGen/entropyGen/internal/common/chclient"
	"github.com/entropyGen/entropyGen/internal/common/giteaclient"
	"github.com/entropyGen/entropyGen/internal/common/redisclient"
)

// Config holds all dependencies for the router.
type Config struct {
	AdminUsername     string
	AdminPasswordHash string
	JWTSecret         []byte
	LiteLLMAddr       string
	LiteLLMMasterKey  string
	AgentNamespace    string
	AgentClient       *k8sclient.AgentClient
	RoleClient        *k8sclient.RoleClient
	CHClient          *chclient.Client
	Pusher            *wspush.Pusher
	GiteaClient       *giteaclient.Client
	StreamWriter      *redisclient.StreamWriter
}

// NewRouter creates and configures the Gin router with all API routes.
func NewRouter(cfg Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	authH := handler.NewAuthHandler(cfg.AdminUsername, cfg.AdminPasswordHash, cfg.JWTSecret)
	agentH := handler.NewAgentHandler(cfg.AgentClient, cfg.GiteaClient, cfg.StreamWriter)
	roleH := handler.NewRoleHandler(cfg.RoleClient)
	llmH := handler.NewLLMHandler(cfg.LiteLLMAddr, cfg.LiteLLMMasterKey)
	auditH := handler.NewAuditHandler(cfg.CHClient)
	wsH := handler.NewWSHandler(cfg.Pusher)

	authMW := handler.JWTMiddleware(cfg.JWTSecret)

	// Public routes
	r.GET("/api/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.POST("/api/auth/login", authH.Login)

	// Protected routes
	api := r.Group("/api", authMW)
	api.GET("/auth/me", authH.Me)
	api.POST("/auth/logout", authH.Logout)

	// Agent management
	api.GET("/agents", agentH.List)
	api.GET("/agents/runtime-images", agentH.RuntimeImages)
	api.POST("/agents", agentH.Create)
	api.GET("/agents/:name", agentH.Get)
	api.PUT("/agents/:name", agentH.Update)
	api.DELETE("/agents/:name", agentH.Delete)
	api.POST("/agents/:name/pause", agentH.Pause)
	api.POST("/agents/:name/resume", agentH.Resume)
	api.POST("/agents/:name/reset-memory", agentH.ResetMemory)
	api.POST("/agents/:name/assign-issue", agentH.AssignIssue)
	api.GET("/agents/:name/logs", agentH.Logs)

	// Role management
	api.GET("/roles", roleH.List)
	api.POST("/roles", roleH.Create)
	api.GET("/roles/:name", roleH.Get)
	api.PATCH("/roles/:name", roleH.Update)
	api.DELETE("/roles/:name", roleH.Delete)
	api.GET("/roles/:name/files", roleH.ListFiles)
	api.GET("/roles/:name/files/:filename", roleH.GetFile)
	api.PUT("/roles/:name/files/:filename", roleH.PutFile)
	api.DELETE("/roles/:name/files/:filename", roleH.DeleteFile)
	api.POST("/roles/:name/files/:filename/rename", roleH.RenameFile)

	// LLM config proxy
	api.GET("/llm/models", llmH.ListModels)
	api.POST("/llm/models", llmH.CreateModel)
	api.PUT("/llm/models/:id", llmH.UpdateModel)
	api.DELETE("/llm/models/:id", llmH.DeleteModel)
	api.GET("/llm/health", llmH.Health)
	api.POST("/llm/health/:id", llmH.HealthModel)
	api.POST("/llm/chat", llmH.Chat)

	// Audit
	api.GET("/audit/traces", auditH.ListTraces)
	api.GET("/audit/traces/:trace_id", auditH.GetTrace)
	api.GET("/audit/stats/token-usage", auditH.TokenUsage)
	api.GET("/audit/stats/agent-activity", auditH.AgentActivity)
	api.GET("/audit/stats/operations", auditH.Operations)
	api.GET("/audit/export", auditH.Export)

	// Agent observation (reverse proxy to sidecar)
	observeH := handler.NewObserveHandler(cfg.AgentNamespace)
	api.Any("/agents/:name/observe/*path", observeH.Proxy)

	// WebSocket
	api.GET("/ws/events", wsH.Handle)

	return r
}
