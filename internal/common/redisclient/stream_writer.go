package redisclient

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/entropyGen/entropyGen/internal/common/models"
)

// MaxLenGateway is the approximate max length for events:gateway stream
const MaxLenGateway = 100000

// MaxLenGitea is the approximate max length for events:gitea stream
const MaxLenGitea = 10000

// MaxLenK8S is the approximate max length for events:k8s stream
const MaxLenK8S = 10000

// StreamWriter writes events to Redis Streams with MAXLEN trimming.
type StreamWriter struct {
	client *redis.Client
}

// NewStreamWriter creates a new StreamWriter backed by the given Redis client.
func NewStreamWriter(client *redis.Client) *StreamWriter {
	return &StreamWriter{client: client}
}

// Write serializes the event to JSON and appends it to the given stream.
// Uses MAXLEN ~ maxLen for approximate trimming (better performance than exact).
// This is a fire-and-forget write; use context cancellation to abort.
func (w *StreamWriter) Write(ctx context.Context, stream string, event *models.Event, maxLen int64) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	return w.client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		MaxLen: maxLen,
		Approx: true,
		Values: map[string]interface{}{
			"data": string(data),
		},
	}).Err()
}
