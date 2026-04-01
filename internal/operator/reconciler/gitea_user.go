package reconciler

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	agentapi "github.com/entropyGen/entropyGen/internal/operator/api"
)

// EnsureGiteaUser creates the Gitea user for the agent's role if not already present.
// Updates agent status with the created username.
func (r *ResourceReconciler) EnsureGiteaUser(ctx context.Context, agent *agentapi.Agent) error {
	username := giteaUsername(agent)
	email := giteaEmail(agent)
	password := generatePassword(agent.Spec.Role)

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
		// Re-fetch to get latest resourceVersion after status update
		if err := r.Client.Get(ctx, client.ObjectKeyFromObject(agent), agent); err != nil {
			return fmt.Errorf("re-fetch agent after status update: %w", err)
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

// EnsureGiteaToken creates a Gitea API token for the role user and stores it in a K8S Secret.
// Secret name: role-{roleName}-gitea-token
// Idempotent: does nothing if the Secret already exists.
// The Secret has no ownerReference because it is shared across agents with the same role.
func (r *ResourceReconciler) EnsureGiteaToken(ctx context.Context, agent *agentapi.Agent) error {
	secretName := fmt.Sprintf("role-%s-gitea-token", agent.Spec.Role)
	existing := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: agent.Namespace}, existing)
	if err == nil {
		return nil // already exists
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("get gitea token secret %q: %w", secretName, err)
	}

	username := giteaUsername(agent)
	password := generatePassword(agent.Spec.Role)
	tokenValue, err := r.GiteaClient.CreateTokenWithPassword(ctx, username, password, "role-api")
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
	return r.Client.Create(ctx, secret)
}

func giteaUsername(agent *agentapi.Agent) string {
	return "role-" + agent.Spec.Role
}

func giteaEmail(agent *agentapi.Agent) string {
	return "role-" + agent.Spec.Role + "@agents.devops.local"
}

// generatePassword returns a random password. Agent uses token auth, not password,
// but Gitea requires a password for user creation and token generation.
func generatePassword(_ string) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 24)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b[i] = chars[n.Int64()]
	}
	return string(b) + "!@1a"
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

// EnsureGiteaRepoAccess adds the role user as a collaborator to each repo in agent.Spec.Gitea.Repos.
// Permission is "write" if the agent has any write/review/merge permission, otherwise "read".
func (r *ResourceReconciler) EnsureGiteaRepoAccess(ctx context.Context, agent *agentapi.Agent) error {
	if agent.Spec.Gitea == nil || len(agent.Spec.Gitea.Repos) == 0 {
		return nil
	}

	username := giteaUsername(agent)
	permission := resolveCollaboratorPermission(agent)

	for _, repo := range agent.Spec.Gitea.Repos {
		parts := strings.SplitN(repo, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			continue
		}
		if err := r.GiteaClient.AddCollaborator(ctx, parts[0], parts[1], username, permission); err != nil {
			return fmt.Errorf("add collaborator %q to %s: %w", username, repo, err)
		}
	}
	return nil
}

// resolveCollaboratorPermission returns "write" if the agent has any write/review/merge permission,
// otherwise "read".
func resolveCollaboratorPermission(agent *agentapi.Agent) string {
	if agent.Spec.Gitea == nil {
		return "read"
	}
	for _, p := range agent.Spec.Gitea.Permissions {
		switch strings.ToLower(p) {
		case "write", "review", "merge":
			return "write"
		}
	}
	return "read"
}
