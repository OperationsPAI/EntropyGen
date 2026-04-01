package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventJSONRoundTrip(t *testing.T) {
	payload := GatewayLLMPayload{
		TraceID:    "trace-123",
		Method:     "POST",
		Path:       "/v1/chat/completions",
		StatusCode: 200,
		Model:      "gpt-4o",
		TokensIn:   1200,
		TokensOut:  350,
		LatencyMs:  2100,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	event := Event{
		EventID:   "evt-001",
		EventType: EventTypeGatewayLLMInference,
		Timestamp: time.Date(2026, 3, 11, 13, 0, 0, 0, time.UTC),
		AgentID:   "agent-developer-1",
		AgentRole: "developer",
		Payload:   json.RawMessage(payloadBytes),
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}

	if decoded.EventID != event.EventID {
		t.Errorf("EventID: got %q, want %q", decoded.EventID, event.EventID)
	}
	if decoded.EventType != event.EventType {
		t.Errorf("EventType: got %q, want %q", decoded.EventType, event.EventType)
	}
	if decoded.AgentID != event.AgentID {
		t.Errorf("AgentID: got %q, want %q", decoded.AgentID, event.AgentID)
	}

	// Verify payload can be extracted
	var decodedPayload GatewayLLMPayload
	if err := json.Unmarshal(decoded.Payload, &decodedPayload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if decodedPayload.Model != payload.Model {
		t.Errorf("payload.Model: got %q, want %q", decodedPayload.Model, payload.Model)
	}
	if decodedPayload.TokensIn != payload.TokensIn {
		t.Errorf("payload.TokensIn: got %d, want %d", decodedPayload.TokensIn, payload.TokensIn)
	}
}
