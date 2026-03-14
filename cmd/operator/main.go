package main

import (
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	agentapi "github.com/entropyGen/entropyGen/internal/operator/api"
	"github.com/entropyGen/entropyGen/internal/operator/controller"
	"github.com/entropyGen/entropyGen/internal/common/giteaclient"
	"github.com/entropyGen/entropyGen/internal/common/redisclient"
	"github.com/entropyGen/entropyGen/internal/operator/scheduler"
)

func main() {
	zapOpts := zap.Options{Development: os.Getenv("DEV_MODE") == "true"}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))
	log := ctrl.Log.WithName("main")

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		log.Error(err, "add client-go scheme")
		os.Exit(1)
	}
	if err := agentapi.AddToScheme(scheme); err != nil {
		log.Error(err, "add agent scheme")
		os.Exit(1)
	}

	giteaURL := mustEnv("GITEA_URL")
	giteaToken := mustEnv("GITEA_ADMIN_TOKEN")
	jwtSecret := mustEnv("JWT_SIGNING_SECRET")
	agentNS := envOr("AGENT_NAMESPACE", "agents")
	controlPlaneNS := envOr("CONTROL_PLANE_NAMESPACE", "control-plane")
	redisAddr := envOr("REDIS_ADDR", "redis.storage.svc:6379")
	gatewayURL := envOr("GATEWAY_URL", fmt.Sprintf("http://agent-gateway.%s.svc:80", controlPlaneNS))
	defaultStorageClass := envOr("DEFAULT_STORAGE_CLASS", "")
	llmAPIKey := envOr("LLM_API_KEY", "")
	llmBaseURL := envOr("LLM_BASE_URL", "")

	giteaCli, err := giteaclient.New(giteaURL, giteaToken)
	if err != nil {
		log.Error(err, "init gitea client")
		os.Exit(1)
	}

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})

	streamWriter := redisclient.NewStreamWriter(rdb)
	cronScheduler := scheduler.New(streamWriter)

	leaseDur := 15 * time.Second
	renewDeadline := 10 * time.Second
	retryPeriod := 2 * time.Second

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		LeaderElection:          true,
		LeaderElectionID:        "devops-operator-leader",
		LeaderElectionNamespace: controlPlaneNS,
		LeaseDuration:           &leaseDur,
		RenewDeadline:           &renewDeadline,
		RetryPeriod:             &retryPeriod,
		HealthProbeBindAddress:  ":8081",
	})
	if err != nil {
		log.Error(err, "init manager")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "add healthz")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "add readyz")
		os.Exit(1)
	}

	reconciler := &controller.AgentReconciler{
		Client:              mgr.GetClient(),
		Scheme:              scheme,
		GiteaClient:         giteaCli,
		JWTSecret:           []byte(jwtSecret),
		AgentNamespace:      agentNS,
		RedisClient:         rdb,
		GatewayURL:          gatewayURL,
		DefaultStorageClass: defaultStorageClass,
		LLMAPIKey:           llmAPIKey,
		LLMBaseURL:          llmBaseURL,
		CronScheduler:       cronScheduler,
		RedisAddr:           redisAddr,
		GiteaURL:            giteaURL,
	}
	if err := reconciler.SetupWithManager(mgr); err != nil {
		log.Error(err, "setup reconciler")
		os.Exit(1)
	}

	log.Info("starting operator")
	cronScheduler.Start()
	defer cronScheduler.Stop()
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "manager exited")
		os.Exit(1)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "required env %s not set\n", key)
		os.Exit(1)
	}
	return v
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
