package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/entropyGen/entropyGen/internal/common/redisclient"
	"github.com/entropyGen/entropyGen/internal/event-collector/k8swatch"
	"github.com/entropyGen/entropyGen/internal/event-collector/webhook"
)

func main() {
	listenAddr := envOr("LISTEN_ADDR", ":8082")
	redisAddr := envOr("REDIS_ADDR", "redis.storage.svc:6379")
	webhookSecret := mustEnv("GITEA_WEBHOOK_SECRET")
	agentNS := envOr("AGENT_NAMESPACE", "agents")

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	writer := redisclient.NewStreamWriter(rdb)
	ctx := context.Background()

	// Start K8S Pod event watcher (gracefully skip if not in cluster)
	watcher, err := k8swatch.NewPodWatcher(agentNS, writer)
	if err != nil {
		slog.Warn("k8s pod watcher unavailable (not in cluster?), skipping", "err", err)
	} else {
		go watcher.Run(ctx)
	}

	giteaHandler := webhook.NewGiteaHandler(webhookSecret, writer)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := rdb.Ping(ctx).Err(); err != nil {
			http.Error(w, "redis unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})
	mux.Handle("POST /webhook/gitea", giteaHandler)

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	slog.Info("event-collector starting", "addr", listenAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		slog.Error("required env not set", "key", k)
		os.Exit(1)
	}
	return v
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
