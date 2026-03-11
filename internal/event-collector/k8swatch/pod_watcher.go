package k8swatch

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/entropyGen/entropyGen/internal/common/models"
	"github.com/entropyGen/entropyGen/internal/common/redisclient"
)

const (
	streamK8S       = "events:k8s"
	k8sStreamMaxLen = int64(10000)
)

// PodWatcher watches Kubernetes Pod Events in the given namespace.
type PodWatcher struct {
	namespace string
	clientset kubernetes.Interface
	writer    *redisclient.StreamWriter
}

// NewPodWatcher creates a PodWatcher using in-cluster kubeconfig.
func NewPodWatcher(namespace string, writer *redisclient.StreamWriter) (*PodWatcher, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("in-cluster config: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("k8s clientset: %w", err)
	}
	return &PodWatcher{namespace: namespace, clientset: cs, writer: writer}, nil
}

// NewPodWatcherWithClient creates a PodWatcher with an injected client (for testing).
func NewPodWatcherWithClient(namespace string, cs kubernetes.Interface, writer *redisclient.StreamWriter) *PodWatcher {
	return &PodWatcher{namespace: namespace, clientset: cs, writer: writer}
}

// Run starts the informer loop. Blocks until ctx is cancelled.
func (w *PodWatcher) Run(ctx context.Context) {
	listWatch := cache.NewListWatchFromClient(
		w.clientset.CoreV1().RESTClient(),
		"events",
		w.namespace,
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		listWatch,
		&corev1.Event{},
		0, // no periodic resync — prevents reprocessing historical events on restart
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				ke, ok := obj.(*corev1.Event)
				if !ok {
					return
				}
				w.HandleEvent(ctx, ke)
			},
			// UpdateFunc deliberately omitted — only new events matter
		},
	)

	slog.Info("k8swatch: pod watcher started", "namespace", w.namespace)
	controller.Run(ctx.Done())
}

// HandleEvent processes a single K8S Event. Only Pod events with recognized
// reasons are written to the events:k8s Redis Stream.
func (w *PodWatcher) HandleEvent(ctx context.Context, ke *corev1.Event) {
	if ke.InvolvedObject.Kind != "Pod" {
		return
	}
	status := podStatusFromReason(ke.Reason)
	if status == "" {
		return
	}

	agentID := agentIDFromPodName(ke.InvolvedObject.Name)

	type payload struct {
		PodName   string `json:"pod_name"`
		Namespace string `json:"namespace"`
		Status    string `json:"status"`
		Reason    string `json:"reason"`
		Message   string `json:"message"`
	}
	p := payload{
		PodName:   ke.InvolvedObject.Name,
		Namespace: ke.InvolvedObject.Namespace,
		Status:    status,
		Reason:    ke.Reason,
		Message:   ke.Message,
	}
	payloadBytes, err := json.Marshal(p)
	if err != nil {
		slog.Warn("k8swatch: marshal payload failed", "err", err)
		return
	}

	event := &models.Event{
		EventID:   uuid.New().String(),
		EventType: models.EventTypeK8SPodStatus,
		Timestamp: time.Now(),
		AgentID:   agentID,
		Payload:   json.RawMessage(payloadBytes),
	}

	if err := w.writer.Write(ctx, streamK8S, event, k8sStreamMaxLen); err != nil {
		slog.Warn("k8swatch: redis write failed",
			"pod", ke.InvolvedObject.Name, "reason", ke.Reason, "err", err)
	}
}

// podStatusFromReason maps a K8S Event Reason to a human-readable status.
func podStatusFromReason(reason string) string {
	switch reason {
	case "Scheduled", "Started", "Pulled", "Created":
		return "Running"
	case "Failed", "BackOff", "FailedMount":
		return "Failed"
	case "OOMKilling":
		return "OOMKilled"
	case "Killing":
		return "Terminating"
	default:
		return ""
	}
}

// agentIDFromPodName strips the Deployment ReplicaSet and Pod hash suffixes.
// Input:  "agent-developer-1-5d8f7-xk9p2"  (format: agent-{name}-{rs}-{pod})
// Output: "agent-developer-1"
func agentIDFromPodName(podName string) string {
	parts := strings.Split(podName, "-")
	// Deployment pods have 2 suffix hash segments; keep everything before them
	if len(parts) >= 4 && parts[0] == "agent" {
		return strings.Join(parts[:len(parts)-2], "-")
	}
	return podName
}
