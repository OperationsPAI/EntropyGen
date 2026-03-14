package controller

import (
	"context"
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
func (r *AgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agentapi.Agent{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.RoleBinding{}).
		Complete(r)
}
