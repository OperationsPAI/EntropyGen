package observer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Config holds the observer server configuration.
type Config struct {
	Port           string
	OpenClawHome   string
	CompletionsDir string
	WorkspaceDir   string
}

// Server is the HTTP server for the agent observer sidecar.
type Server struct {
	cfg    Config
	router *gin.Engine
	wsHub  *WSHub
}

// NewServer creates and configures the observer HTTP server.
func NewServer(cfg Config, wsHub *WSHub) *Server {
	s := &Server{
		cfg:   cfg,
		wsHub: wsHub,
	}
	s.router = s.setupRouter()
	return s
}

// Run starts the HTTP server on the configured port.
func (s *Server) Run() error {
	return s.router.Run(":" + s.cfg.Port)
}

// Router returns the underlying gin.Engine (useful for testing).
func (s *Server) Router() *gin.Engine {
	return s.router
}

// setupRouter creates the Gin router with all routes.
func (s *Server) setupRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/healthz", s.handleHealthz)
	r.GET("/sessions", s.handleListSessions)
	r.GET("/sessions/current", s.handleCurrentSession)
	r.GET("/sessions/:id", s.handleGetSession)
	r.GET("/workspace/tree", s.handleWorkspaceTree)
	r.GET("/workspace/file", s.handleWorkspaceFile)
	r.GET("/workspace/diff", s.handleWorkspaceDiff)
	r.GET("/ws/live", s.handleWSLive)

	return r
}

func (s *Server) handleHealthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) handleListSessions(c *gin.Context) {
	sessions, err := ListSessions(s.cfg.CompletionsDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("LIST_FAILED", err.Error()))
		return
	}
	c.JSON(http.StatusOK, sessions)
}

func (s *Server) handleCurrentSession(c *gin.Context) {
	lines, _, err := ReadCurrentSession(s.cfg.CompletionsDir)
	if err != nil {
		c.JSON(http.StatusNotFound, apiError("NO_CURRENT_SESSION", err.Error()))
		return
	}
	writeNDJSON(c, lines)
}

func (s *Server) handleGetSession(c *gin.Context) {
	sessionID := c.Param("id")
	lines, err := ReadSessionContent(s.cfg.CompletionsDir, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, apiError("SESSION_NOT_FOUND", fmt.Sprintf("session %q not found", sessionID)))
		return
	}
	writeNDJSON(c, lines)
}

func (s *Server) handleWorkspaceTree(c *gin.Context) {
	tree, err := BuildFileTree(s.cfg.WorkspaceDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("TREE_FAILED", err.Error()))
		return
	}
	c.JSON(http.StatusOK, tree)
}

func (s *Server) handleWorkspaceFile(c *gin.Context) {
	reqPath := c.Query("path")
	if reqPath == "" {
		c.JSON(http.StatusBadRequest, apiError("MISSING_PATH", "query parameter 'path' is required"))
		return
	}

	content, err := ReadFile(s.cfg.WorkspaceDir, reqPath)
	if err != nil {
		if strings.Contains(err.Error(), "path traversal denied") {
			c.JSON(http.StatusForbidden, apiError("PATH_TRAVERSAL", "access denied"))
			return
		}
		c.JSON(http.StatusNotFound, apiError("FILE_NOT_FOUND", err.Error()))
		return
	}
	c.JSON(http.StatusOK, content)
}

func (s *Server) handleWorkspaceDiff(c *gin.Context) {
	diff := GetGitDiff(s.cfg.WorkspaceDir)
	c.JSON(http.StatusOK, diff)
}

func (s *Server) handleWSLive(c *gin.Context) {
	s.wsHub.HandleUpgrade(c.Writer, c.Request)
}

// writeNDJSON writes a slice of raw JSON messages as newline-delimited JSON.
func writeNDJSON(c *gin.Context, lines []json.RawMessage) {
	c.Header("Content-Type", "application/x-ndjson")
	c.Status(http.StatusOK)
	for _, line := range lines {
		fmt.Fprintln(c.Writer, string(line))
	}
}

// apiError returns the standard error response format.
func apiError(code, msg string) gin.H {
	return gin.H{"error": msg, "code": code}
}
