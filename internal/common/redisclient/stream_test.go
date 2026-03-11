package redisclient_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/entropyGen/entropyGen/internal/common/models"
	"github.com/entropyGen/entropyGen/internal/common/redisclient"
)

func newTestClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return client, mr
}

func makeTestEvent(eventType string) *models.Event {
	return &models.Event{
		EventID:   "test-evt-001",
		EventType: eventType,
		Timestamp: time.Now(),
		AgentID:   "agent-test-1",
		AgentRole: "developer",
		Payload:   json.RawMessage(`{"key":"value"}`),
	}
}

func TestStreamWriter_Write(t *testing.T) {
	client, _ := newTestClient(t)
	writer := redisclient.NewStreamWriter(client)
	ctx := context.Background()

	event := makeTestEvent(models.EventTypeGatewayHeartbeat)
	if err := writer.Write(ctx, "test:stream", event, 1000); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Verify message was added
	msgs, err := client.XLen(ctx, "test:stream").Result()
	if err != nil {
		t.Fatalf("XLen: %v", err)
	}
	if msgs != 1 {
		t.Errorf("expected 1 message, got %d", msgs)
	}
}

func TestStreamReader_ReadAndACK(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()
	const stream = "test:events"
	const group = "test-group"
	const consumer = "test-consumer-1"

	writer := redisclient.NewStreamWriter(client)
	reader := redisclient.NewStreamReader(client, group, consumer)

	// Write one message first
	event := makeTestEvent(models.EventTypeGatewayHeartbeat)
	if err := writer.Write(ctx, stream, event, 1000); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Create group starting from beginning
	if err := client.XGroupCreateMkStream(ctx, stream, group, "0").Err(); err != nil &&
		err.Error() != "BUSYGROUP Consumer Group name already exists" {
		t.Fatalf("CreateGroup: %v", err)
	}

	// Read messages
	msgs, err := reader.Read(ctx, stream, 10, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Event.EventID != event.EventID {
		t.Errorf("EventID: got %q, want %q", msgs[0].Event.EventID, event.EventID)
	}

	// ACK the message
	if err := reader.ACK(ctx, stream, msgs[0].ID); err != nil {
		t.Fatalf("ACK: %v", err)
	}

	// Read again - should get nothing (already ACKed)
	msgs2, err := reader.Read(ctx, stream, 10, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Read2: %v", err)
	}
	if len(msgs2) != 0 {
		t.Errorf("expected 0 pending messages after ACK, got %d", len(msgs2))
	}
}
