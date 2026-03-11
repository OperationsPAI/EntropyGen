package chclient_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/entropyGen/entropyGen/internal/common/chclient"
)

func getTestClient(t *testing.T) *chclient.Client {
	t.Helper()
	addr := os.Getenv("CLICKHOUSE_ADDR")
	if addr == "" {
		addr = "localhost:9000"
	}
	// Check if ClickHouse is available
	client, err := chclient.New(addr, "audit", "default", "")
	if err != nil {
		t.Skipf("ClickHouse not available at %s: %v", addr, err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestCreateSchema_Idempotent(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Run twice to verify idempotency
	if err := client.CreateSchema(ctx); err != nil {
		t.Fatalf("CreateSchema (1st): %v", err)
	}
	if err := client.CreateSchema(ctx); err != nil {
		t.Fatalf("CreateSchema (2nd, idempotent): %v", err)
	}
}

func TestInsertAndQuery(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	if err := client.CreateSchema(ctx); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	traces := []chclient.AuditTrace{
		{
			TraceID:     uuid.New().String(),
			SpanID:      uuid.New().String(),
			AgentID:     "agent-test-1",
			AgentRole:   "developer",
			RequestType: "llm_inference",
			Method:      "POST",
			Path:        "/v1/chat/completions",
			StatusCode:  200,
			Model:       "gpt-4o",
			TokensIn:    100,
			TokensOut:   50,
			LatencyMs:   500,
			CreatedAt:   time.Now(),
		},
		{
			TraceID:     uuid.New().String(),
			SpanID:      uuid.New().String(),
			AgentID:     "agent-test-1",
			AgentRole:   "developer",
			RequestType: "gitea_api",
			Method:      "GET",
			Path:        "/api/v1/repos",
			StatusCode:  200,
			LatencyMs:   45,
			CreatedAt:   time.Now(),
		},
		{
			TraceID:     uuid.New().String(),
			SpanID:      uuid.New().String(),
			AgentID:     "agent-test-1",
			AgentRole:   "developer",
			RequestType: "heartbeat",
			Method:      "GET",
			Path:        "/healthz",
			StatusCode:  200,
			LatencyMs:   5,
			CreatedAt:   time.Now(),
		},
	}

	if err := client.InsertTraces(ctx, traces); err != nil {
		t.Fatalf("InsertTraces: %v", err)
	}

	// Wait a moment for ClickHouse to process
	time.Sleep(100 * time.Millisecond)

	results, err := client.GetRecentTraces(ctx, "agent-test-1", 10)
	if err != nil {
		t.Fatalf("GetRecentTraces: %v", err)
	}
	if len(results) < 3 {
		t.Errorf("expected at least 3 traces, got %d", len(results))
	}
}
