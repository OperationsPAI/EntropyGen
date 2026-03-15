package controller

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/redis/go-redis/v9"

	agentapi "github.com/entropyGen/entropyGen/internal/operator/api"
	"github.com/entropyGen/entropyGen/internal/common/giteaclient"
	"github.com/entropyGen/entropyGen/internal/operator/reconciler"
	"github.com/entropyGen/entropyGen/internal/operator/scheduler"
)

const (
	FinalizerName  = "aidevops.io/cleanup"
	requeueOnError = 30 * time.Second
)

// AgentReconciler reconciles Agent CRs.
type AgentReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	GiteaClient    *giteaclient.Client
	JWTSecret      []byte
	AgentNamespace string
	RedisClient    *redis.Client
	GatewayURL          string
	DefaultStorageClass string
	LLMAPIKey           string
	LLMBaseURL          string
	CronScheduler       *scheduler.CronScheduler
	RedisAddr           string
	GiteaURL            string
	RolesDataPath       string

	// roleChangeCh receives events from the role filesystem watcher.
	roleChangeCh chan event.GenericEvent
}

func (r *AgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	agent := &agentapi.Agent{}
	if err := r.Get(ctx, req.NamespacedName, agent); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !agent.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(agent, FinalizerName) {
			log.Info("cleaning up agent resources", "name", agent.Name)
			if r.CronScheduler != nil {
				r.CronScheduler.Remove(agent.Name)
			}
			res := r.newResourceReconciler()
			if err := res.DeleteAll(ctx, agent); err != nil {
				log.Error(err, "cleanup failed")
				return ctrl.Result{RequeueAfter: requeueOnError}, err
			}
			controllerutil.RemoveFinalizer(agent, FinalizerName)
			return ctrl.Result{}, r.Update(ctx, agent)
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer
	if !controllerutil.ContainsFinalizer(agent, FinalizerName) {
		controllerutil.AddFinalizer(agent, FinalizerName)
		if err := r.Update(ctx, agent); err != nil {
			return ctrl.Result{}, err
		}
		// Re-fetch after update to get latest resourceVersion
		if err := r.Get(ctx, req.NamespacedName, agent); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Set Initializing phase on first reconcile
	if agent.Status.Phase == "" {
		agent.Status.Phase = "Initializing"
		_ = r.Status().Update(ctx, agent)
	}

	// Reconcile all resources
	res := r.newResourceReconciler()
	if err := res.ReconcileAll(ctx, agent); err != nil {
		log.Error(err, "reconcile failed")
		r.setCondition(agent, "Ready", "False", "ReconcileError", err.Error())
		_ = r.Status().Update(ctx, agent)
		return ctrl.Result{RequeueAfter: requeueOnError}, err
	}

	// Update status
	if !agent.Spec.Paused {
		agent.Status.Phase = "Running"
	} else {
		agent.Status.Phase = "Paused"
	}
	r.setCondition(agent, "Ready", "True", "Reconciled", "All resources reconciled successfully")
	_ = r.Status().Update(ctx, agent)

	// Sync cron scheduler
	if r.CronScheduler != nil {
		if agent.Spec.Paused {
			r.CronScheduler.Remove(agent.Name)
		} else if agent.Spec.Cron != nil && agent.Spec.Cron.Schedule != "" {
			prompt := res.CronPrompt(agent)
			r.CronScheduler.Sync(agent.Name, agent.Spec.Cron.Schedule, prompt)
		} else {
			r.CronScheduler.Remove(agent.Name)
		}
	}

	log.Info("reconcile complete", "phase", agent.Status.Phase)
	return ctrl.Result{}, nil
}

func (r *AgentReconciler) newResourceReconciler() *reconciler.ResourceReconciler {
	return &reconciler.ResourceReconciler{
		Client:              r.Client,
		Scheme:              r.Scheme,
		GiteaClient:         r.GiteaClient,
		JWTSecret:           r.JWTSecret,
		RedisClient:         r.RedisClient,
		GatewayURL:          r.GatewayURL,
		DefaultStorageClass: r.DefaultStorageClass,
		LLMAPIKey:           r.LLMAPIKey,
		LLMBaseURL:          r.LLMBaseURL,
		RedisAddr:           r.RedisAddr,
		GiteaURL:            r.GiteaURL,
		RolesDataPath:       r.RolesDataPath,
	}
}

func (r *AgentReconciler) setCondition(agent *agentapi.Agent, condType, status, reason, message string) {
	now := metav1.Now()
	for i, c := range agent.Status.Conditions {
		if c.Type == condType {
			agent.Status.Conditions[i] = agentapi.AgentCondition{
				Type:               condType,
				Status:             status,
				Reason:             reason,
				Message:            message,
				LastTransitionTime: now,
			}
			return
		}
	}
	agent.Status.Conditions = append(agent.Status.Conditions, agentapi.AgentCondition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
	})
}

// SetupWithManager registers the reconciler and owned resource types.
// Watches the roles-data PVC directory for changes via a filesystem scanner.
func (r *AgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.roleChangeCh = make(chan event.GenericEvent, 64)

	// Start background role filesystem watcher
	if err := mgr.Add(&roleWatcher{
		client:        mgr.GetClient(),
		rolesDataPath: r.RolesDataPath,
		agentNS:       r.AgentNamespace,
		ch:            r.roleChangeCh,
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&agentapi.Agent{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.RoleBinding{}).
		WatchesRawSource(source.Channel(r.roleChangeCh, &handler.EnqueueRequestForObject{})).
		Complete(r)
}

// roleMetadata mirrors the .metadata.json structure from the backend.
type roleMetadata struct {
	UpdatedAt time.Time `json:"updated_at"`
}

// roleWatcher periodically scans the roles PVC directory for changes
// and emits GenericEvents to trigger agent reconciliation.
type roleWatcher struct {
	client        client.Client
	rolesDataPath string
	agentNS       string
	ch            chan event.GenericEvent

	mu            sync.Mutex
	lastUpdatedAt map[string]time.Time // roleName → last known updated_at
}

func (w *roleWatcher) Start(ctx context.Context) error {
	w.lastUpdatedAt = make(map[string]time.Time)
	log := ctrl.Log.WithName("role-watcher")

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			changedRoles := w.scanForChanges()
			if len(changedRoles) == 0 {
				continue
			}
			log.Info("role changes detected", "roles", changedRoles)
			for _, roleName := range changedRoles {
				w.enqueueAgentsForRole(ctx, roleName)
			}
		}
	}
}

func (w *roleWatcher) scanForChanges() []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	entries, err := os.ReadDir(w.rolesDataPath)
	if err != nil {
		return nil
	}

	var changed []string
	seen := make(map[string]bool)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		seen[name] = true

		metaPath := filepath.Join(w.rolesDataPath, name, ".metadata.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta roleMetadata
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		if prev, ok := w.lastUpdatedAt[name]; !ok || !meta.UpdatedAt.Equal(prev) {
			w.lastUpdatedAt[name] = meta.UpdatedAt
			// Skip on first scan (initial population)
			if ok {
				changed = append(changed, name)
			}
		}
	}
	// Clean up deleted roles
	for name := range w.lastUpdatedAt {
		if !seen[name] {
			delete(w.lastUpdatedAt, name)
			changed = append(changed, name)
		}
	}
	return changed
}

func (w *roleWatcher) enqueueAgentsForRole(ctx context.Context, roleName string) {
	var agents agentapi.AgentList
	if err := w.client.List(ctx, &agents, client.InNamespace(w.agentNS)); err != nil {
		return
	}
	for _, a := range agents.Items {
		if a.Spec.Role == roleName {
			w.ch <- event.GenericEvent{
				Object: &agentapi.Agent{
					ObjectMeta: metav1.ObjectMeta{
						Name:      a.Name,
						Namespace: a.Namespace,
					},
				},
			}
		}
	}
}
