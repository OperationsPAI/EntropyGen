package api

import (
	"github.com/gin-gonic/gin"

	"github.com/entropyGen/entropyGen/internal/backend/handler"
	"github.com/entropyGen/entropyGen/internal/backend/k8sclient"
	"github.com/entropyGen/entropyGen/internal/backend/wspush"
	"github.com/entropyGen/entropyGen/internal/common/chclient"
	"github.com/entropyGen/entropyGen/internal/common/giteaclient"
	"github.com/entropyGen/entropyGen/internal/common/pgclient"
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
	GiteaBaseURL      string
	PGClient          *pgclient.Client // optional; nil disables user management
}

// NewRouter creates and configures the Gin router with all API routes.
func NewRouter(cfg Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// UserStore: use PGClient when available, else nil (DB-less mode)
	var userStore handler.UserStore
	if cfg.PGClient != nil {
		userStore = cfg.PGClient
	}

	authH := handler.NewAuthHandler(cfg.AdminUsername, cfg.AdminPasswordHash, cfg.JWTSecret, userStore)
	agentH := handler.NewAgentHandler(cfg.AgentClient, cfg.GiteaClient, cfg.StreamWriter, cfg.AgentNamespace)
	agentH.SetClickHouse(cfg.CHClient)
	roleH := handler.NewRoleHandler(cfg.RoleClient)
	llmH := handler.NewLLMHandler(cfg.LiteLLMAddr, cfg.LiteLLMMasterKey)
	auditH := handler.NewAuditHandler(cfg.CHClient)
	wsH := handler.NewWSHandler(cfg.Pusher)
	configH := handler.NewConfigHandler(cfg.GiteaBaseURL)
	observeH := handler.NewObserveHandler(cfg.AgentNamespace)

	// ── Group 1: Fully public (no auth required) ─────────────────────────────
	r.GET("/api/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.GET("/api/config", configH.Get)
	r.POST("/api/auth/login", authH.Login)

	// ── Group 2 & 3: Optional JWT ─────────────────────────────────────────────
	// OptionalJWTMiddleware: sets role="guest" if no/invalid token, never rejects
	opt := r.Group("/api", handler.OptionalJWTMiddleware(cfg.JWTSecret))

	// Auth
	opt.GET("/auth/me", handler.RequireRole("member", "admin"), authH.Me)
	opt.POST("/auth/logout", handler.RequireRole("member", "admin"), authH.Logout)

	// Agent management — read (guest+)
	opt.GET("/agents", agentH.List)
	opt.GET("/agents/:name", agentH.Get)
	opt.GET("/agents/:name/logs", agentH.Logs)

	// Agent management — write (member+)
	opt.GET("/agents/runtime-images", handler.RequireRole("member", "admin"), agentH.RuntimeImages)
	opt.POST("/agents", handler.RequireRole("member", "admin"), agentH.Create)
	opt.PUT("/agents/:name", handler.RequireRole("member", "admin"), agentH.Update)
	opt.DELETE("/agents/:name", handler.RequireRole("member", "admin"), agentH.Delete)
	opt.POST("/agents/:name/pause", handler.RequireRole("member", "admin"), agentH.Pause)
	opt.POST("/agents/:name/resume", handler.RequireRole("member", "admin"), agentH.Resume)
	opt.POST("/agents/:name/reset-memory", handler.RequireRole("member", "admin"), agentH.ResetMemory)
	opt.POST("/agents/:name/assign-issue", handler.RequireRole("member", "admin"), agentH.AssignIssue)

	// Role management — read (member+)
	opt.GET("/roles/types", handler.RequireRole("member", "admin"), roleH.ListTypes)
	opt.GET("/roles", handler.RequireRole("member", "admin"), roleH.List)
	opt.GET("/roles/:name", handler.RequireRole("member", "admin"), roleH.Get)
	opt.GET("/roles/:name/files", handler.RequireRole("member", "admin"), roleH.ListFiles)
	opt.GET("/roles/:name/files/*filepath", handler.RequireRole("member", "admin"), roleH.GetFile)
	opt.GET("/roles/:name/validate", handler.RequireRole("member", "admin"), roleH.Validate)
	opt.GET("/roles/:name/export", handler.RequireRole("member", "admin"), roleH.Export)

	// Role management — write (member+)
	opt.POST("/roles", handler.RequireRole("member", "admin"), roleH.Create)
	opt.PATCH("/roles/:name", handler.RequireRole("member", "admin"), roleH.Update)
	opt.DELETE("/roles/:name", handler.RequireRole("member", "admin"), roleH.Delete)
	opt.PUT("/roles/:name/files/*filepath", handler.RequireRole("member", "admin"), roleH.PutFile)
	opt.DELETE("/roles/:name/files/*filepath", handler.RequireRole("member", "admin"), roleH.DeleteFile)
	opt.POST("/roles/:name/rename-file", handler.RequireRole("member", "admin"), roleH.RenameFile)

	// LLM — read (member+)
	opt.GET("/llm/models", handler.RequireRole("member", "admin"), llmH.ListModels)
	opt.GET("/llm/health", handler.RequireRole("member", "admin"), llmH.Health)

	// LLM — member write
	opt.POST("/llm/chat", handler.RequireRole("member", "admin"), llmH.Chat)
	opt.POST("/llm/health/:id", handler.RequireRole("member", "admin"), llmH.HealthModel)

	// LLM model management — admin only
	opt.POST("/llm/models", handler.RequireRole("admin"), llmH.CreateModel)
	opt.PUT("/llm/models/:id", handler.RequireRole("admin"), llmH.UpdateModel)
	opt.DELETE("/llm/models/:id", handler.RequireRole("admin"), llmH.DeleteModel)

	// Audit — read (member+), export (member+)
	opt.GET("/audit/traces", handler.RequireRole("member", "admin"), auditH.ListTraces)
	opt.GET("/audit/traces/:trace_id", handler.RequireRole("member", "admin"), auditH.GetTrace)
	opt.GET("/audit/stats/token-usage", handler.RequireRole("member", "admin"), auditH.TokenUsage)
	opt.GET("/audit/stats/agent-activity", handler.RequireRole("member", "admin"), auditH.AgentActivity)
	opt.GET("/audit/stats/operations", handler.RequireRole("member", "admin"), auditH.Operations)
	opt.GET("/audit/export", handler.RequireRole("member", "admin"), auditH.Export)

	// Monitor (member+)
	opt.GET("/monitor/token-trend", handler.RequireRole("member", "admin"), auditH.TokenUsage)
	opt.GET("/monitor/activity-heatmap", handler.RequireRole("member", "admin"), auditH.AgentActivity)
	opt.GET("/monitor/model-distribution", handler.RequireRole("member", "admin"), auditH.ModelDistribution)
	opt.GET("/monitor/latency-trend", handler.RequireRole("member", "admin"), auditH.LatencyTrend)
	opt.GET("/monitor/agent-ranking", handler.RequireRole("member", "admin"), auditH.AgentRanking)

	// Agent observation (member+)
	opt.Any("/agents/:name/observe/*path", handler.RequireRole("member", "admin"), observeH.Proxy)

	// WebSocket (member+)
	opt.GET("/ws/events", handler.RequireRole("member", "admin"), wsH.Handle)

	// User management — admin only (only when PGClient configured)
	if userStore != nil {
		userH := handler.NewUserHandler(userStore)
		opt.GET("/users", handler.RequireRole("admin"), userH.List)
		opt.POST("/users", handler.RequireRole("admin"), userH.Create)
		opt.PUT("/users/:username", handler.RequireRole("admin"), userH.Update)
		opt.DELETE("/users/:username", handler.RequireRole("admin"), userH.Delete)
	}

	return r
}
