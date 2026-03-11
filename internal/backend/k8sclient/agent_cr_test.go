package k8sclient_test

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	agentapi "github.com/entropyGen/entropyGen/internal/operator/api"
	"github.com/entropyGen/entropyGen/internal/backend/k8sclient"
)

func newTestAgentClient(t *testing.T) *k8sclient.AgentClient {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = agentapi.AddToScheme(scheme)
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&agentapi.Agent{}).Build()
	return k8sclient.NewAgentClient(c, "agents")
}

func TestAgentClient_CreateAndList(t *testing.T) {
	c := newTestAgentClient(t)
	ctx := context.Background()

	_, err := c.Create(ctx, "developer-1", agentapi.AgentSpec{
		Role: "developer",
		Soul: "You are a developer agent.",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	agents, err := c.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Name != "developer-1" {
		t.Errorf("name: got %q, want developer-1", agents[0].Name)
	}
}

func TestAgentClient_Update(t *testing.T) {
	c := newTestAgentClient(t)
	ctx := context.Background()

	c.Create(ctx, "test-agent", agentapi.AgentSpec{Role: "observer", Soul: "original soul"})

	updated, err := c.Update(ctx, "test-agent", agentapi.AgentSpec{Role: "observer", Soul: "new soul"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Spec.Soul != "new soul" {
		t.Errorf("soul: got %q, want 'new soul'", updated.Spec.Soul)
	}
}

func TestAgentClient_SetPaused(t *testing.T) {
	c := newTestAgentClient(t)
	ctx := context.Background()

	c.Create(ctx, "test-agent", agentapi.AgentSpec{Role: "observer", Soul: "soul"})

	agent, err := c.SetPaused(ctx, "test-agent", true)
	if err != nil {
		t.Fatalf("SetPaused(true): %v", err)
	}
	if !agent.Spec.Paused {
		t.Error("expected agent paused=true")
	}

	agent, err = c.SetPaused(ctx, "test-agent", false)
	if err != nil {
		t.Fatalf("SetPaused(false): %v", err)
	}
	if agent.Spec.Paused {
		t.Error("expected agent paused=false")
	}
}

func TestAgentClient_Delete(t *testing.T) {
	c := newTestAgentClient(t)
	ctx := context.Background()

	c.Create(ctx, "to-delete", agentapi.AgentSpec{Role: "observer", Soul: "soul"})

	if err := c.Delete(ctx, "to-delete"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	agents, _ := c.List(ctx)
	if len(agents) != 0 {
		t.Errorf("expected 0 agents after delete, got %d", len(agents))
	}
}

func TestAgentClient_NilClient_Error(t *testing.T) {
	c := k8sclient.NewAgentClient(nil, "agents")
	_, err := c.List(context.Background())
	if err == nil {
		t.Error("expected error with nil client")
	}
}
