package heartbeat

import (
	"testing"
)

// Detector.runOnce requires real ClickHouse, K8s, and Redis connections.
// Unit tests will be added once interfaces are extracted for these dependencies.
// See architecture review: chclient, k8sclient, and redisclient need consumer-side interfaces.
func TestDetector_TODO(t *testing.T) {
	t.Skip("requires interface extraction for chclient, k8sclient, redisclient")
}
