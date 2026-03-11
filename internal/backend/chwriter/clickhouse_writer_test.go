package chwriter

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/entropyGen/entropyGen/internal/common/models"
)

func TestEventToTrace_GatewayLLM(t *testing.T) {
	payload, _ := json.Marshal(map[string]interface{}{
		"method":        "POST",
		"path":          "/v1/chat/completions",
		"status_code":   200,
		"model":         "gpt-4o",
		"tokens_in":     1200,
		"tokens_out":    350,
		"latency_ms":    2100,
		"request_body":  `{"messages":[]}`,
		"response_body": `{"choices":[]}`,
	})
	e := &models.Event{
		EventID:   "e1",
		EventType: "gateway.llm_inference",
		Timestamp: time.Now(),
		AgentID:   "agent-dev-1",
		AgentRole: "developer",
		Payload:   json.RawMessage(payload),
	}
	trace := EventToTrace(e)

	if trace.AgentID != "agent-dev-1" {
		t.Errorf("AgentID: got %q, want agent-dev-1", trace.AgentID)
	}
	if trace.RequestType != "llm_inference" {
		t.Errorf("RequestType: got %q, want llm_inference", trace.RequestType)
	}
	if trace.Model != "gpt-4o" {
		t.Errorf("Model: got %q, want gpt-4o", trace.Model)
	}
	if trace.TokensIn != 1200 {
		t.Errorf("TokensIn: got %d, want 1200", trace.TokensIn)
	}
	if trace.StatusCode != 200 {
		t.Errorf("StatusCode: got %d, want 200", trace.StatusCode)
	}
}

func TestEventToTrace_GiteaEvent(t *testing.T) {
	payload, _ := json.Marshal(map[string]interface{}{
		"repo":         "org/platform-demo",
		"issue_number": 42,
	})
	e := &models.Event{
		EventType: "gitea.issue_open",
		AgentID:   "",
		Payload:   json.RawMessage(payload),
	}
	trace := EventToTrace(e)
	if trace.RequestType != "issue_open" {
		t.Errorf("RequestType: got %q, want issue_open", trace.RequestType)
	}
}

func TestEventToTrace_NilPayload(t *testing.T) {
	e := &models.Event{EventType: "k8s.pod_status"}
	trace := EventToTrace(e) // should not panic
	if trace.RequestType != "pod_status" {
		t.Errorf("RequestType: got %q, want pod_status", trace.RequestType)
	}
}
