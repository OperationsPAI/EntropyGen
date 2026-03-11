package heartbeat

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/entropyGen/entropyGen/internal/backend/k8sclient"
	"github.com/entropyGen/entropyGen/internal/common/chclient"
	"github.com/entropyGen/entropyGen/internal/common/models"
	"github.com/entropyGen/entropyGen/internal/common/redisclient"
)

const heartbeatTimeout = 15 * time.Minute

// Detector periodically checks for agents with stale heartbeats.
type Detector struct {
	ch           *chclient.Client
	agentClient  *k8sclient.AgentClient
	streamWriter *redisclient.StreamWriter
}

// NewDetector creates a heartbeat detector.
func NewDetector(ch *chclient.Client, ac *k8sclient.AgentClient, sw *redisclient.StreamWriter) *Detector {
	return &Detector{ch: ch, agentClient: ac, streamWriter: sw}
}

// RunLoop starts the periodic heartbeat check until ctx is cancelled.
func (d *Detector) RunLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.detect(ctx)
		}
	}
}

func (d *Detector) detect(ctx context.Context) {
	timeouts, err := d.ch.GetHeartbeatTimeouts(ctx, heartbeatTimeout)
	if err != nil {
		slog.Warn("heartbeat detector: query failed", "err", err)
		return
	}
	for _, to := range timeouts {
		slog.Warn("heartbeat timeout", "agent_id", to.AgentID, "last", to.LastHeartbeat)

		// Patch agent CR status to Error
		agentName := to.AgentID
		if len(agentName) > 6 && agentName[:6] == "agent-" {
			agentName = agentName[6:]
		}
		if err := d.agentClient.SetStatusPhase(ctx, agentName, "Error"); err != nil {
			slog.Warn("heartbeat detector: set phase failed", "agent", agentName, "err", err)
		}

		// Write alert event
		payload, _ := json.Marshal(map[string]interface{}{
			"alert_type":     models.AlertTypeHeartbeatTimeout,
			"agent_id":       to.AgentID,
			"last_heartbeat": to.LastHeartbeat.Format(time.RFC3339),
			"message":        "Agent heartbeat timeout: no heartbeat in 15 minutes",
		})
		_ = d.streamWriter.Write(ctx, "events:k8s", &models.Event{
			EventID:   uuid.New().String(),
			EventType: models.EventTypeOperatorAgentAlert,
			Timestamp: time.Now(),
			AgentID:   to.AgentID,
			Payload:   json.RawMessage(payload),
		}, 10000)
	}
}
