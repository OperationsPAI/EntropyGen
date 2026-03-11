package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/entropyGen/entropyGen/internal/gateway/audit"
	"github.com/entropyGen/entropyGen/internal/gateway/gatewayctx"
	"github.com/entropyGen/entropyGen/internal/gateway/handler"
)

func TestHeartbeatHandler_OK(t *testing.T) {
	ew := audit.NewNopEventWriter()
	hb := handler.NewHeartbeatHandler(ew)

	ctx := context.WithValue(context.Background(), gatewayctx.AgentID, "agent-test-1")
	ctx = context.WithValue(ctx, gatewayctx.AgentRole, "observer")
	req := httptest.NewRequest("POST", "/heartbeat", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	hb.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status field: got %q, want ok", resp["status"])
	}
}
