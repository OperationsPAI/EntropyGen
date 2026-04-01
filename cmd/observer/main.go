package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/entropyGen/entropyGen/internal/common/redisclient"
	"github.com/entropyGen/entropyGen/internal/observer"
)

func main() {
	port := envOr("OBSERVER_PORT", "8081")
	workspaceDir := envOr("WORKSPACE_DIR", "/workspace")

	// Ensure directory exists
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

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func ensureDir(dir string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Warn("cannot create directory", "dir", dir, "err", err)
	}
}
