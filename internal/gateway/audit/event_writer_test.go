package audit

import (
	"strings"
	"testing"

	"github.com/entropyGen/entropyGen/internal/common/models"
)

func TestBuildPayload_LLMWithBodies(t *testing.T) {
	rec := RequestRecord{
		TraceID:   "t1",
		EventType: "gateway.llm_inference",
		Method:    "POST",
		Path:      "/v1/chat/completions",
		Status:    200,
		LatencyMs: 1500,
		ReqBody:   []byte(`{"model":"gpt-4o","messages":[]}`),
		RespBody:  []byte(`{"choices":[{"message":{"content":"hi"}}]}`),
		IsLLM:     true,
	}
	p := buildPayload(rec)

	if p["request_body"] == "" {
		t.Error("LLM event should have non-empty request_body")
	}
	if p["response_body"] == "" {
		t.Error("LLM event should have non-empty response_body")
	}
	if p["_body_truncated"] != false {
		t.Errorf("small body should not be truncated, got %v", p["_body_truncated"])
	}
	if p["status_code"] != 200 {
		t.Errorf("status_code: got %v, want 200", p["status_code"])
	}
	if p["latency_ms"] != int64(1500) {
		t.Errorf("latency_ms: got %v, want 1500", p["latency_ms"])
	}
}

func TestBuildPayload_NonLLM(t *testing.T) {
	rec := RequestRecord{
		EventType: "gateway.gitea_api",
		Method:    "GET",
		Path:      "/api/v1/repos",
		Status:    200,
		ReqBody:   []byte("should be ignored"),
		IsLLM:     false,
	}
	p := buildPayload(rec)

	if p["request_body"] != "" {
		t.Errorf("non-LLM should have empty request_body, got %q", p["request_body"])
	}
	if p["response_body"] != "" {
		t.Errorf("non-LLM should have empty response_body, got %q", p["response_body"])
	}
}

func TestTruncateBody_Oversized(t *testing.T) {
	big := []byte(strings.Repeat("x", maxBodyBytes+500))
	result, truncated := truncateBody(big)
	if !truncated {
		t.Error("oversized body should be truncated")
	}
	if !strings.HasSuffix(result, "...[TRUNCATED]") {
		t.Error("truncated body should end with ...[TRUNCATED]")
	}
	// result length should not exceed maxBodyBytes + len("...[TRUNCATED]")
	if len(result) > maxBodyBytes+20 {
		t.Errorf("truncated result too long: %d", len(result))
	}
}

func TestTruncateBody_Empty(t *testing.T) {
	result, truncated := truncateBody(nil)
	if result != "" || truncated {
		t.Errorf("empty body: got %q truncated=%v, want '' false", result, truncated)
	}
}

func TestEventWriter_EnqueueDropsWhenFull(t *testing.T) {
	ew := NewNopEventWriter()
	// Fill channel to capacity
	for i := 0; i < channelSize; i++ {
		ew.Enqueue(&models.Event{EventType: "test"})
	}
	// One more should not panic or block
	ew.Enqueue(&models.Event{EventType: "dropped"})
}

func TestEventWriter_EnqueueRequest(t *testing.T) {
	ew := NewNopEventWriter()
	ew.EnqueueRequest(RequestRecord{
		TraceID:   "r1",
		EventType: "gateway.gitea_api",
		AgentID:   "agent-1",
		Method:    "GET",
		Path:      "/api/v1/repos",
		Status:    200,
		LatencyMs: 45,
	})
	// Should have one event in channel
	if len(ew.ch) != 1 {
		t.Errorf("expected 1 event in channel, got %d", len(ew.ch))
	}
}
