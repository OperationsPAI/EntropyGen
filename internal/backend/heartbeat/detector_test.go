package heartbeat

import (
	"testing"
	"time"

	"github.com/entropyGen/entropyGen/internal/common/chclient"
)

func TestHeartbeatTimeout_Constant(t *testing.T) {
	if heartbeatTimeout != 15*time.Minute {
		t.Errorf("heartbeatTimeout: got %v, want 15m", heartbeatTimeout)
	}
}

func TestDetector_New(t *testing.T) {
	// Verify constructor doesn't panic with nil deps
	d := NewDetector(nil, nil, nil)
	if d == nil {
		t.Error("NewDetector returned nil")
	}
}

func TestEventToTrace_Placeholder(t *testing.T) {
	// Verify the chclient types are accessible from this package
	now := time.Now()
	trace := chclient.AuditTrace{
		AgentID:   "agent-test",
		CreatedAt: now,
	}
	if trace.AgentID != "agent-test" {
		t.Errorf("AgentID: got %q", trace.AgentID)
	}
}
