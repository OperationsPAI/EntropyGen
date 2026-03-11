package redisclient

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/entropyGen/entropyGen/internal/common/models"
)

// StreamMessage wraps a Redis XMessage with the parsed Event.
type StreamMessage struct {
	ID    string
	Event *models.Event
}

// StreamReader reads from Redis Streams using Consumer Groups.
type StreamReader struct {
	client   *redis.Client
	group    string
	consumer string
}

// NewStreamReader creates a StreamReader that reads as the given consumer in the given group.
func NewStreamReader(client *redis.Client, group, consumer string) *StreamReader {
	return &StreamReader{client: client, group: group, consumer: consumer}
}

// CreateGroup creates the consumer group if it doesn't exist.
// Uses $ to start from the latest message (skip historical events).
func (r *StreamReader) CreateGroup(ctx context.Context, stream string) error {
	err := r.client.XGroupCreateMkStream(ctx, stream, r.group, "$").Err()
	if err != nil && err.Error() == "BUSYGROUP Consumer Group name already exists" {
		return nil
	}
	return err
}

// Read fetches up to count messages from the stream, blocking for up to blockDuration.
// Returns parsed StreamMessages. Unrecognized payloads are included with Event.Payload as raw JSON.
func (r *StreamReader) Read(ctx context.Context, stream string, count int64, blockDuration time.Duration) ([]StreamMessage, error) {
	result, err := r.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    r.group,
		Consumer: r.consumer,
		Streams:  []string{stream, ">"},
		Count:    count,
		Block:    blockDuration,
	}).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("XREADGROUP %s: %w", stream, err)
	}

	var messages []StreamMessage
	for _, s := range result {
		for _, msg := range s.Messages {
			sm, parseErr := parseXMessage(msg)
			if parseErr != nil {
				continue // skip malformed messages
			}
			messages = append(messages, sm)
		}
	}
	return messages, nil
}

// ACK acknowledges the given message IDs, removing them from the pending list.
func (r *StreamReader) ACK(ctx context.Context, stream string, ids ...string) error {
	return r.client.XAck(ctx, stream, r.group, ids...).Err()
}

func parseXMessage(msg redis.XMessage) (StreamMessage, error) {
	data, ok := msg.Values["data"]
	if !ok {
		return StreamMessage{}, fmt.Errorf("message %s missing 'data' field", msg.ID)
	}
	dataStr, ok := data.(string)
	if !ok {
		return StreamMessage{}, fmt.Errorf("message %s 'data' field is not a string", msg.ID)
	}
	var event models.Event
	if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
		return StreamMessage{}, fmt.Errorf("unmarshal event from message %s: %w", msg.ID, err)
	}
	return StreamMessage{ID: msg.ID, Event: &event}, nil
}
