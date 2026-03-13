package reconciler

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	agentapi "github.com/entropyGen/entropyGen/internal/operator/api"
	"github.com/entropyGen/entropyGen/internal/common/giteaclient"
)

// ResourceReconciler handles reconciliation of all K8S and external resources for an Agent CR.
type ResourceReconciler struct {
	Client      client.Client
	Scheme      *runtime.Scheme
	GiteaClient *giteaclient.Client
	JWTSecret   []byte
	RedisClient *redis.Client
	GatewayURL          string
	DefaultStorageClass string
	LLMAPIKey           string
	LLMBaseURL          string

	// roleData is populated by EnsureRoleData and consumed by subsequent steps.
	roleData *roleData
}

// EnsureRoleData fetches the Role ConfigMap and caches the parsed data.
func (r *ResourceReconciler) EnsureRoleData(ctx context.Context, agent *agentapi.Agent) error {
	rd, err := r.fetchRoleData(ctx, agent)
	if err != nil {
		return err
	}
	r.roleData = rd
	return nil
}

// ReconcileAll reconciles resources in dependency order (7-step sequence from design doc S3).
func (r *ResourceReconciler) ReconcileAll(ctx context.Context, agent *agentapi.Agent) error {
	type step struct {
		name string
		fn   func(context.Context, *agentapi.Agent) error
	}
	steps := []step{
		{"gitea-user", r.EnsureGiteaUser},
		{"jwt-secret", r.EnsureJWTSecret},
		{"fetch-role", r.EnsureRoleData},
		{"configmap", r.EnsureConfigMap},
		{"skills-configmap", r.EnsureSkillsConfigMap},
		{"role-files-configmap", r.EnsureRoleFilesConfigMap},
		{"pvc", r.EnsurePVC},
		{"serviceaccount", r.EnsureServiceAccount},
		{"rolebinding", r.EnsureRoleBinding},
		{"deployment", r.EnsureDeployment},
	}
	for _, s := range steps {
		if err := s.fn(ctx, agent); err != nil {
			return fmt.Errorf("step %s: %w", s.name, err)
		}
	}
	return nil
}

// DeleteAll cleans up external resources (K8S resources are cleaned via ownerReference cascade).
func (r *ResourceReconciler) DeleteAll(ctx context.Context, agent *agentapi.Agent) error {
	return r.DeleteGiteaUser(ctx, agent)
}
