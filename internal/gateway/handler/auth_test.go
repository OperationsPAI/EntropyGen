package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/entropyGen/entropyGen/internal/gateway/audit"
	"github.com/entropyGen/entropyGen/internal/gateway/gatewayctx"
	"github.com/entropyGen/entropyGen/internal/gateway/handler"
)

var testSecret = []byte("test-secret-32-bytes-long-enough")

func makeTestToken(t *testing.T, agentID, role string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":        agentID,
		"agent_id":   agentID,
		"agent_role": role,
		"iat":        time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(testSecret)
	if err != nil {
		t.Fatalf("make token: %v", err)
	}
	return signed
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	ew := audit.NewNopEventWriter()
	mw := handler.NewAuthMiddleware(testSecret, ew)

	var gotAgentID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAgentID, _ = r.Context().Value(gatewayctx.AgentID).(string)
		w.WriteHeader(http.StatusOK)
	})

	tokenStr := makeTestToken(t, "agent-dev-1", "developer")
	req := httptest.NewRequest("GET", "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	mw.Wrap(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if gotAgentID != "agent-dev-1" {
		t.Errorf("agent_id in context: got %q, want agent-dev-1", gotAgentID)
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	ew := audit.NewNopEventWriter()
	mw := handler.NewAuthMiddleware(testSecret, ew)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v1/models", nil)
	rec := httptest.NewRecorder()
	mw.Wrap(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	ew := audit.NewNopEventWriter()
	mw := handler.NewAuthMiddleware(testSecret, ew)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	rec := httptest.NewRecorder()
	mw.Wrap(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestAuthMiddleware_ExpZeroToken(t *testing.T) {
	ew := audit.NewNopEventWriter()
	mw := handler.NewAuthMiddleware(testSecret, ew)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	claims := jwt.MapClaims{
		"sub":        "agent-test",
		"agent_id":   "agent-test",
		"agent_role": "observer",
		"iat":        time.Now().Unix(),
		"exp":        int64(0),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString(testSecret)

	req := httptest.NewRequest("GET", "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rec := httptest.NewRecorder()
	mw.Wrap(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("exp=0 token should be accepted, got %d", rec.Code)
	}
}
