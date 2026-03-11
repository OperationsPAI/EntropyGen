package commands

import (
	"fmt"
	"os"
	"strings"

	sdk "code.gitea.io/sdk/gitea"
)

// newClient creates a Gitea SDK client using token from file and base URL from env.
func newClient() (*sdk.Client, error) {
	tokenPath := os.Getenv("GITEA_TOKEN_PATH")
	if tokenPath == "" {
		tokenPath = "/agent/secrets/gitea-token"
	}
	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("read token from %s: %w", tokenPath, err)
	}
	token := strings.TrimSpace(string(tokenBytes))

	baseURL := os.Getenv("GITEA_BASE_URL")
	if baseURL == "" {
		baseURL = "http://agent-gateway.control-plane.svc"
	}

	return sdk.NewClient(baseURL, sdk.SetToken(token))
}

// splitRepo splits "org/repo" into (owner, repo, error).
func splitRepo(repo string) (string, string, error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("repo must be in org/repo format, got: %s", repo)
	}
	return parts[0], parts[1], nil
}
