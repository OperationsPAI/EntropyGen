package reconciler

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	agentapi "github.com/entropyGen/entropyGen/internal/operator/api"
)

// EnsureGiteaUser creates the Gitea user for the agent if not already present.
// Updates agent status with the created username.
func (r *ResourceReconciler) EnsureGiteaUser(ctx context.Context, agent *agentapi.Agent) error {
	username := giteaUsername(agent)
	email := giteaEmail(agent)
	password := generatePassword(agent.Name)

	err := r.GiteaClient.CreateUser(ctx, username, email, password)
	if err != nil && !isGiteaAlreadyExists(err) {
		return fmt.Errorf("create gitea user %q: %w", username, err)
	}

	// Update status so backend can resolve the gitea username.
	if agent.Status.GiteaUser == nil || agent.Status.GiteaUser.Username != username {
		if agent.Status.GiteaUser == nil {
			agent.Status.GiteaUser = &agentapi.GiteaUserStatus{}
		}
		agent.Status.GiteaUser.Created = true
		agent.Status.GiteaUser.Username = username
		if err := r.Client.Status().Update(ctx, agent); err != nil {
			return fmt.Errorf("update gitea user status: %w", err)
		}
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

// EnsureGiteaToken creates a Gitea API token for the agent and stores it in a K8S Secret.
// Secret name: agent-{name}-gitea-token
// Idempotent: does nothing if the Secret already exists.
func (r *ResourceReconciler) EnsureGiteaToken(ctx context.Context, agent *agentapi.Agent) error {
	secretName := fmt.Sprintf("agent-%s-gitea-token", agent.Name)
	existing := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: agent.Namespace}, existing)
	if err == nil {
		return nil // already exists
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("get gitea token secret %q: %w", secretName, err)
	}

	username := giteaUsername(agent)
	password := generatePassword(agent.Name)
	tokenValue, err := r.GiteaClient.CreateTokenWithPassword(ctx, username, password, "agent-api")
	if err != nil {
		return fmt.Errorf("create gitea token for %q: %w", username, err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: agent.Namespace,
		},
		StringData: map[string]string{"token": tokenValue},
	}
	_ = controllerutil.SetControllerReference(agent, secret, r.Scheme)
	return r.Client.Create(ctx, secret)
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
