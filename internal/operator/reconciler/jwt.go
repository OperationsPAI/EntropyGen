package reconciler

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	agentapi "github.com/entropyGen/entropyGen/internal/operator/api"
)

// EnsureJWTSecret signs a JWT token for the agent and stores it in a K8S Secret.
// Secret name: agent-{name}-jwt-token
// Idempotent: does nothing if the Secret already exists.
func (r *ResourceReconciler) EnsureJWTSecret(ctx context.Context, agent *agentapi.Agent) error {
	secretName := fmt.Sprintf("agent-%s-jwt-token", agent.Name)
	existing := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: agent.Namespace}, existing)
	if err == nil {
		return nil // already exists
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("get jwt secret %q: %w", secretName, err)
	}

	token, err := IssueAgentJWT(fmt.Sprintf("agent-%s", agent.Name), agent.Spec.Role, r.JWTSecret)
	if err != nil {
		return fmt.Errorf("issue jwt for %q: %w", agent.Name, err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: agent.Namespace,
		},
		StringData: map[string]string{"token": token},
	}
	_ = controllerutil.SetControllerReference(agent, secret, r.Scheme)
	return r.Client.Create(ctx, secret)
}

// IssueAgentJWT signs a JWT token using HS256 with the given signing secret.
// Claims: sub, agent_id, agent_role, iat (no expiry per design spec).
func IssueAgentJWT(agentID, agentRole string, signingSecret []byte) (string, error) {
	claims := jwt.MapClaims{
		"sub":        agentID,
		"agent_id":   agentID,
		"agent_role": agentRole,
		"iat":        time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(signingSecret)
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signed, nil
}
