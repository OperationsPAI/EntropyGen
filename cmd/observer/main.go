package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/entropyGen/entropyGen/internal/common/redisclient"
	"github.com/entropyGen/entropyGen/internal/observer"
)

func main() {
	port := envOr("OBSERVER_PORT", "8081")
	openClawHome := expandHome(envOr("OPENCLAW_HOME", "~/.openclaw"))
	completionsDir := envOr("COMPLETIONS_DIR", filepath.Join(openClawHome, "completions"))
	workspaceDir := envOr("WORKSPACE_DIR", filepath.Join(openClawHome, "workspace"))

	// Ensure directories exist
	ensureDir(completionsDir)
	ensureDir(workspaceDir)

	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	cfg := observer.Config{
		Port:           port,
		OpenClawHome:   openClawHome,
		CompletionsDir: completionsDir,
		WorkspaceDir:   workspaceDir,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	watcher := observer.NewWatcher(completionsDir, workspaceDir)
	wsHub := observer.NewWSHub(watcher)

	go func() {
		if err := watcher.Run(ctx); err != nil && ctx.Err() == nil {
			slog.Error("watcher failed", "err", err)
		}
	}()
	go wsHub.Run(ctx, completionsDir)

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
	slog.Info("observer starting", "addr", ":"+port, "home", openClawHome)
	if err := srv.Run(); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func expandHome(path string) string {
	if len(path) > 1 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func ensureDir(dir string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Warn("cannot create directory", "dir", dir, "err", err)
	}
}
