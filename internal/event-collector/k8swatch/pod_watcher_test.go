package k8swatch_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/entropyGen/entropyGen/internal/common/redisclient"
	"github.com/entropyGen/entropyGen/internal/event-collector/k8swatch"
)

type testEnv struct {
	watcher *k8swatch.PodWatcher
	rdb     *redis.Client
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	writer := redisclient.NewStreamWriter(rdb)
	cs := fake.NewSimpleClientset()
	w := k8swatch.NewPodWatcherWithClient("agents", cs, writer)
	return &testEnv{watcher: w, rdb: rdb}
}

func streamLen(ctx context.Context, rdb *redis.Client, stream string) int64 {
	n, _ := rdb.XLen(ctx, stream).Result()
	return n
}

func TestHandleEvent_OOMKilling(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	ke := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oom-event-1",
			Namespace: "agents",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      "agent-developer-1-abc12-xyz45",
			Namespace: "agents",
		},
		Reason:  "OOMKilling",
		Message: "Container exceeded memory limit",
	}
	env.watcher.HandleEvent(ctx, ke)

	n := streamLen(ctx, env.rdb, "events:k8s")
	if n != 1 {
		t.Fatalf("expected 1 event in events:k8s, got %d", n)
	}

	// Verify payload content
	msgs, err := env.rdb.XRange(ctx, "events:k8s", "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange: %v", err)
	}
	data, ok := msgs[0].Values["data"].(string)
	if !ok {
		t.Fatal("data field not a string")
	}
	var evt struct {
		EventType string          `json:"event_type"`
		AgentID   string          `json:"agent_id"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal([]byte(data), &evt); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if evt.EventType != "k8s.pod_status" {
		t.Errorf("event_type: got %q, want %q", evt.EventType, "k8s.pod_status")
	}
	if evt.AgentID != "agent-developer-1" {
		t.Errorf("agent_id: got %q, want %q", evt.AgentID, "agent-developer-1")
	}

	var p struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(evt.Payload, &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if p.Status != "OOMKilled" {
		t.Errorf("payload.status: got %q, want %q", p.Status, "OOMKilled")
	}
	if p.Reason != "OOMKilling" {
		t.Errorf("payload.reason: got %q, want %q", p.Reason, "OOMKilling")
	}
}

func TestHandleEvent_NonPodEvent_Ignored(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	ke := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "dep-event", Namespace: "agents"},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Deployment",
			Name: "agent-developer-1",
		},
		Reason: "ScalingReplicaSet",
	}
	env.watcher.HandleEvent(ctx, ke)

	if n := streamLen(ctx, env.rdb, "events:k8s"); n != 0 {
		t.Errorf("Deployment events should be ignored, got %d messages", n)
	}
}

func TestHandleEvent_UnknownReason_Ignored(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	ke := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "misc-event", Namespace: "agents"},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod",
			Name: "agent-sre-1-abc12-xyz45",
		},
		Reason: "Pulling",
	}
	env.watcher.HandleEvent(ctx, ke)

	if n := streamLen(ctx, env.rdb, "events:k8s"); n != 0 {
		t.Errorf("unknown reason should be ignored, got %d messages", n)
	}
}

func TestHandleEvent_AllReasons(t *testing.T) {
	tests := []struct {
		reason     string
		wantStatus string
	}{
		{"Scheduled", "Running"},
		{"Started", "Running"},
		{"Pulled", "Running"},
		{"Created", "Running"},
		{"Failed", "Failed"},
		{"BackOff", "Failed"},
		{"FailedMount", "Failed"},
		{"OOMKilling", "OOMKilled"},
		{"Killing", "Terminating"},
	}
	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			env := newTestEnv(t)
			ctx := context.Background()

			ke := &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{Name: "evt-" + tt.reason, Namespace: "agents"},
				InvolvedObject: corev1.ObjectReference{
					Kind:      "Pod",
					Name:      "agent-dev-1-abc12-xyz45",
					Namespace: "agents",
				},
				Reason: tt.reason,
			}
			env.watcher.HandleEvent(ctx, ke)

			if n := streamLen(ctx, env.rdb, "events:k8s"); n != 1 {
				t.Fatalf("expected 1 event for reason %q, got %d", tt.reason, n)
			}

			msgs, _ := env.rdb.XRange(ctx, "events:k8s", "-", "+").Result()
			data := msgs[0].Values["data"].(string)
			var evt struct {
				Payload json.RawMessage `json:"payload"`
			}
			json.Unmarshal([]byte(data), &evt)
			var p struct {
				Status string `json:"status"`
			}
			json.Unmarshal(evt.Payload, &p)
			if p.Status != tt.wantStatus {
				t.Errorf("reason %q: got status %q, want %q", tt.reason, p.Status, tt.wantStatus)
			}
		})
	}
}

func TestHandleEvent_AgentIDParsing(t *testing.T) {
	tests := []struct {
		podName     string
		wantAgentID string
	}{
		{"agent-developer-1-abc12-xyz45", "agent-developer-1"},
		{"agent-sre-1-abc12-xyz45", "agent-sre-1"},
		{"agent-observer-1-abc12-xyz45", "agent-observer-1"},
		{"some-other-pod-abc12-xyz45", "some-other-pod-abc12-xyz45"}, // non-agent pod
	}
	for _, tt := range tests {
		t.Run(tt.podName, func(t *testing.T) {
			env := newTestEnv(t)
			ctx := context.Background()

			ke := &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{Name: "evt", Namespace: "agents"},
				InvolvedObject: corev1.ObjectReference{
					Kind:      "Pod",
					Name:      tt.podName,
					Namespace: "agents",
				},
				Reason: "OOMKilling",
			}
			env.watcher.HandleEvent(ctx, ke)

			msgs, _ := env.rdb.XRange(ctx, "events:k8s", "-", "+").Result()
			data := msgs[0].Values["data"].(string)
			var evt struct {
				AgentID string `json:"agent_id"`
			}
			json.Unmarshal([]byte(data), &evt)
			if evt.AgentID != tt.wantAgentID {
				t.Errorf("podName %q: got agent_id %q, want %q", tt.podName, evt.AgentID, tt.wantAgentID)
			}
		})
	}
}
