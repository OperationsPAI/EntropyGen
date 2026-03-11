package redisclient

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Claim re-assigns pending messages that have been idle for longer than minIdleTime.
// Useful for recovering messages after a consumer crash/restart.
// startID is the stream entry ID to start claiming from (use "0-0" to start from beginning).
func (r *StreamReader) Claim(ctx context.Context, stream string, minIdleTime time.Duration, startID string, count int64) ([]StreamMessage, error) {
	xMessages, _, err := r.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   stream,
		Group:    r.group,
		Consumer: r.consumer,
		MinIdle:  minIdleTime,
		Start:    startID,
		Count:    count,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("XAUTOCLAIM %s: %w", stream, err)
	}

	var messages []StreamMessage
	for _, msg := range xMessages {
		sm, parseErr := parseXMessage(msg)
		if parseErr != nil {
			continue
		}
		messages = append(messages, sm)
	}
	return messages, nil
}
