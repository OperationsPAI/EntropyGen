package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/entropyGen/entropyGen/internal/common/models"
	"github.com/entropyGen/entropyGen/internal/gateway/audit"
	"github.com/entropyGen/entropyGen/internal/gateway/gatewayctx"
)

// HeartbeatHandler handles POST /heartbeat from Agent Runtimes.
type HeartbeatHandler struct {
	eventWriter *audit.EventWriter
}

// NewHeartbeatHandler creates a new HeartbeatHandler.
func NewHeartbeatHandler(ew *audit.EventWriter) *HeartbeatHandler {
	return &HeartbeatHandler{eventWriter: ew}
}

// ServeHTTP processes a heartbeat request and emits an audit event.
func (h *HeartbeatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	agentID, _ := r.Context().Value(gatewayctx.AgentID).(string)
	agentRole, _ := r.Context().Value(gatewayctx.AgentRole).(string)
	traceID := uuid.New().String()
	now := time.Now()

	payload, _ := json.Marshal(map[string]string{
		"trace_id": traceID,
		"status":   "ok",
	})
	h.eventWriter.Enqueue(&models.Event{
		EventID:   uuid.New().String(),
		EventType: models.EventTypeGatewayHeartbeat,
		Timestamp: now,
		AgentID:   agentID,
		AgentRole: agentRole,
		Payload:   payload,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "ok",
		"timestamp": now.UTC().Format(time.RFC3339),
	})
}
