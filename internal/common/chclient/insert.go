package chclient

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AuditTrace represents a row in audit.traces table.
type AuditTrace struct {
	TraceID      string    `ch:"trace_id" json:"trace_id"`
	SpanID       string    `ch:"span_id" json:"span_id"`
	AgentID      string    `ch:"agent_id" json:"agent_id"`
	AgentRole    string    `ch:"agent_role" json:"agent_role"`
	RequestType  string    `ch:"request_type" json:"request_type"`
	Method       string    `ch:"method" json:"method"`
	Path         string    `ch:"path" json:"path"`
	StatusCode   uint16    `ch:"status_code" json:"status_code"`
	RequestBody  string    `ch:"request_body" json:"request_body"`
	ResponseBody string    `ch:"response_body" json:"response_body"`
	ToolCalls    string    `ch:"tool_calls" json:"tool_calls"`
	Model        string    `ch:"model" json:"model"`
	TokensIn     uint32    `ch:"tokens_in" json:"tokens_in"`
	TokensOut    uint32    `ch:"tokens_out" json:"tokens_out"`
	LatencyMs    uint32    `ch:"latency_ms" json:"latency_ms"`
	CreatedAt    time.Time `ch:"created_at" json:"created_at"`
}

// NewAuditTrace creates an AuditTrace with generated UUIDs and current timestamp.
func NewAuditTrace() AuditTrace {
	return AuditTrace{
		TraceID:   uuid.New().String(),
		SpanID:    uuid.New().String(),
		CreatedAt: time.Now(),
	}
}

// InsertTraces batch-inserts audit traces into ClickHouse.
func (c *Client) InsertTraces(ctx context.Context, traces []AuditTrace) error {
	if len(traces) == 0 {
		return nil
	}

	batch, err := c.conn.PrepareBatch(ctx, "INSERT INTO audit.traces")
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}

	for _, t := range traces {
		if err := batch.AppendStruct(&t); err != nil {
			return fmt.Errorf("append trace %s: %w", t.TraceID, err)
		}
	}

	return batch.Send()
}
