package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	agentapi "github.com/entropyGen/entropyGen/internal/operator/api"

	"github.com/entropyGen/entropyGen/internal/backend/api"
	"github.com/entropyGen/entropyGen/internal/backend/builtin"
	"github.com/entropyGen/entropyGen/internal/backend/chwriter"
	"github.com/entropyGen/entropyGen/internal/backend/heartbeat"
	"github.com/entropyGen/entropyGen/internal/backend/k8sclient"
	"github.com/entropyGen/entropyGen/internal/backend/wspush"
	"github.com/entropyGen/entropyGen/internal/common/chclient"
	"github.com/entropyGen/entropyGen/internal/common/giteaclient"
	"github.com/entropyGen/entropyGen/internal/common/pgclient"
	"github.com/entropyGen/entropyGen/internal/common/redisclient"
)

func main() {
	listenAddr := envOr("LISTEN_ADDR", ":8081")
	chAddr := mustEnv("CLICKHOUSE_ADDR")
	chDB := envOr("CLICKHOUSE_DB", "audit")
	chUser := envOr("CLICKHOUSE_USER", "default")
	chPass := envOr("CLICKHOUSE_PASS", "")
	redisAddr := envOr("REDIS_ADDR", "redis.storage.svc:6379")
	litellmAddr := mustEnv("LITELLM_ADDR")
	litellmKey := envOr("LITELLM_MASTER_KEY", "")
	adminUser := mustEnv("ADMIN_USERNAME")
	adminPassHash := mustEnv("ADMIN_PASSWORD_HASH")
	jwtSecret := []byte(mustEnv("JWT_SECRET"))
	agentNS := envOr("AGENT_NAMESPACE", "agents")
	dlqDir := envOr("DLQ_DIR", "/var/lib/backend/dlq")
	giteaURL := envOr("GITEA_URL", "")
	giteaExternalURL := envOr("GITEA_EXTERNAL_URL", giteaURL)
	giteaToken := envOr("GITEA_ADMIN_TOKEN", "")
	rolesDataPath := envOr("ROLES_DATA_PATH", "/data/roles")
	databaseURL := envOr("DATABASE_URL", "")

	// Gitea (optional: assign-issue endpoint requires it)
	var giteaCli *giteaclient.Client
	if giteaURL != "" && giteaToken != "" {
		var err error
		giteaCli, err = giteaclient.New(giteaURL, giteaToken)
		if err != nil {
			slog.Error("gitea client init failed", "err", err)
			os.Exit(1)
		}
		slog.Info("gitea client initialized", "url", giteaURL)
	} else {
		slog.Warn("GITEA_URL or GITEA_ADMIN_TOKEN not set, assign-issue endpoint disabled")
	}

	// PostgreSQL (optional: RBAC user store)
	var pgClient *pgclient.Client
	if databaseURL != "" {
		var err error
		pgClient, err = pgclient.New(databaseURL)
		if err != nil {
			slog.Error("postgres init failed", "err", err)
			os.Exit(1)
		}
		defer pgClient.Close()
		slog.Info("postgres connected")

		// Seed admin user on first boot
		seedCtx := context.Background()
		if pgClient.CountUsers(seedCtx) == 0 {
			_, seedErr := pgClient.CreateUser(seedCtx, pgclient.CreateUserInput{
				Username:     adminUser,
				PasswordHash: adminPassHash,
				Role:         "admin",
			})
			if seedErr != nil {
				slog.Error("failed to seed admin user", "err", seedErr)
			} else {
				slog.Info("seeded admin user", "username", adminUser)
			}
		}
	} else {
		slog.Warn("DATABASE_URL not set, user management disabled (falling back to env-based auth)")
	}

	// ClickHouse
	ch, err := chclient.New(chAddr, chDB, chUser, chPass)
	if err != nil {
		slog.Error("clickhouse init failed", "err", err)
		os.Exit(1)
	}
	if err := ch.CreateSchema(context.Background()); err != nil {
		slog.Error("clickhouse schema init failed", "err", err)
		os.Exit(1)
	}

	// Redis
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	streamWriter := redisclient.NewStreamWriter(rdb)
	chReader := redisclient.NewStreamReader(rdb, "ch-writer", "backend-1")
	wsReader := redisclient.NewStreamReader(rdb, "ws-push", "backend-ws-1")

	ctx := context.Background()
	for _, stream := range []string{"events:gateway", "events:gitea", "events:k8s"} {
		_ = chReader.CreateGroup(ctx, stream)
		_ = wsReader.CreateGroup(ctx, stream)
	}

	// K8S
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = agentapi.AddToScheme(scheme)
	var agentCRClient *k8sclient.AgentClient
	var k8sClientset kubernetes.Interface
	if k8sCfg, err := ctrlconfig.GetConfig(); err == nil {
		var k8sClient ctrlclient.Client
		if c, err := ctrlclient.New(k8sCfg, ctrlclient.Options{Scheme: scheme}); err == nil {
			k8sClient = c
		}
		if cs, err := kubernetes.NewForConfig(k8sCfg); err == nil {
			k8sClientset = cs
		}
		if k8sClient != nil {
			agentCRClient = k8sclient.NewAgentClientWithKube(k8sClient, k8sClientset, k8sCfg, agentNS)
		}
	}
	if agentCRClient == nil {
		slog.Warn("k8s unavailable, agent API disabled")
		agentCRClient = k8sclient.NewAgentClient(nil, agentNS)
	}
	roleClient := k8sclient.NewRoleClient(rolesDataPath, agentCRClient, &builtin.Provider{})

	// Background services
	pusher := wspush.NewPusher()
	go pusher.Run(ctx, wsReader)

	cw := chwriter.New(ch, chReader, streamWriter, dlqDir)
	go cw.Run(ctx)

	detector := heartbeat.NewDetector(ch, agentCRClient, streamWriter)
	go detector.RunLoop(ctx, 5*time.Minute)

	// HTTP server
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := api.NewRouter(api.Config{
		AdminUsername:     adminUser,
		AdminPasswordHash: adminPassHash,
		JWTSecret:         jwtSecret,
		LiteLLMAddr:       litellmAddr,
		LiteLLMMasterKey:  litellmKey,
		AgentNamespace:    agentNS,
		AgentClient:       agentCRClient,
		RoleClient:        roleClient,
		CHClient:          ch,
		Pusher:            pusher,
		GiteaClient:       giteaCli,
		StreamWriter:      streamWriter,
		GiteaBaseURL:      giteaExternalURL,
		PGClient:          pgClient,
	})
	slog.Info("backend starting", "addr", listenAddr)
	if err := router.Run(listenAddr); err != nil {
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
