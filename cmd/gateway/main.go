package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/entropyGen/entropyGen/internal/common/redisclient"
	"github.com/entropyGen/entropyGen/internal/gateway/audit"
	"github.com/entropyGen/entropyGen/internal/gateway/handler"
	"github.com/entropyGen/entropyGen/internal/gateway/ratelimit"
)

// Config holds all gateway configuration read from environment variables.
type Config struct {
	ListenAddr   string
	LiteLLMAddr  string
	GiteaAddr    string
	RedisAddr    string
	JWTSecret    []byte
	RateLimitRPM int
	BurstSize    int
}

func main() {
	cfg := Config{
		ListenAddr:   envOr("LISTEN_ADDR", ":8080"),
		LiteLLMAddr:  mustEnv("LITELLM_ADDR"),
		GiteaAddr:    mustEnv("GITEA_ADDR"),
		RedisAddr:    envOr("REDIS_ADDR", "redis.storage.svc:6379"),
		JWTSecret:    []byte(mustEnv("JWT_SECRET")),
		RateLimitRPM: 60,
		BurstSize:    10,
	}

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	writer := redisclient.NewStreamWriter(rdb)
	eventWriter := audit.NewEventWriter(writer)
	go eventWriter.Run(context.Background())

	limiter := ratelimit.NewLimiter(cfg.RateLimitRPM, cfg.BurstSize)
	authMW := handler.NewAuthMiddleware(cfg.JWTSecret, eventWriter)
	proxy := handler.NewProxyHandler(cfg.LiteLLMAddr, cfg.GiteaAddr, eventWriter)
	hb := handler.NewHeartbeatHandler(eventWriter)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("POST /heartbeat", authMW.Wrap(limiter.Wrap(hb)))
	mux.Handle("/v1/", authMW.Wrap(limiter.Wrap(proxy)))
	mux.Handle("/api/v1/", authMW.Wrap(limiter.Wrap(proxy)))
	mux.Handle("/git/", authMW.Wrap(limiter.Wrap(proxy)))

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second, // git clone needs up to 300s
		IdleTimeout:  120 * time.Second,
	}
	slog.Info("gateway starting", "addr", cfg.ListenAddr)
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
