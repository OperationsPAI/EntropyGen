package main

import (
	"context"
	"os"

	"go.uber.org/zap"

	giteainit "github.com/entropyGen/entropyGen/internal/gitea-init/init"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	cfg := giteainit.Config{
		GiteaURL:             getEnv("GITEA_URL", "http://gitea.gitea.svc:3000"),
		AdminToken:           mustEnv("GITEA_ADMIN_TOKEN", log),
		WebhookSecret:        mustEnv("WEBHOOK_SECRET", log),
		OrgName:              getEnv("GITEA_ORG", "platform"),
		RepoName:             getEnv("GITEA_REPO", "platform-demo"),
		EventCollectorURL:    mustEnv("EVENT_COLLECTOR_URL", log),
		RunnerTokenNamespace: getEnv("RUNNER_TOKEN_NAMESPACE", "gitea"),
	}

	runner, err := giteainit.NewRunner(cfg, log)
	if err != nil {
		log.Fatal("init runner", zap.Error(err))
	}

	if err := runner.Run(context.Background()); err != nil {
		log.Fatal("init failed", zap.Error(err))
	}

	log.Info("gitea initialization completed successfully")
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func mustEnv(key string, log *zap.Logger) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatal("required env var not set", zap.String("key", key))
	}
	return v
}
