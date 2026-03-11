package reconciler

import (
	"context"
	"fmt"
	"strings"

	agentapi "github.com/entropyGen/entropyGen/internal/operator/api"
)

// EnsureGiteaUser creates the Gitea user for the agent if not already present.
func (r *ResourceReconciler) EnsureGiteaUser(ctx context.Context, agent *agentapi.Agent) error {
	username := giteaUsername(agent)
	email := giteaEmail(agent)
	password := generatePassword(agent.Name)

	err := r.GiteaClient.CreateUser(ctx, username, email, password)
	if err != nil && !isGiteaAlreadyExists(err) {
		return fmt.Errorf("create gitea user %q: %w", username, err)
	}
	return nil
}

// DeleteGiteaUser removes the Gitea user for the agent.
func (r *ResourceReconciler) DeleteGiteaUser(ctx context.Context, agent *agentapi.Agent) error {
	username := giteaUsername(agent)
	if err := r.GiteaClient.DeleteUser(ctx, username); err != nil && !isGiteaNotFound(err) {
		return fmt.Errorf("delete gitea user %q: %w", username, err)
	}
	return nil
}

func giteaUsername(agent *agentapi.Agent) string {
	if agent.Spec.Gitea != nil && agent.Spec.Gitea.Username != "" {
		return "agent-" + agent.Spec.Gitea.Username
	}
	return "agent-" + agent.Name
}

func giteaEmail(agent *agentapi.Agent) string {
	if agent.Spec.Gitea != nil && agent.Spec.Gitea.Email != "" {
		return agent.Spec.Gitea.Email
	}
	return giteaUsername(agent) + "@agents.devops.local"
}

// generatePassword returns a deterministic placeholder password (agent uses token auth, not password).
func generatePassword(name string) string {
	return "AgentP@ss-" + strings.ReplaceAll(name, "-", "") + "!"
}

func isGiteaAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "already exists") || strings.Contains(s, "422")
}

func isGiteaNotFound(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "not found") || strings.Contains(s, "404")
}
