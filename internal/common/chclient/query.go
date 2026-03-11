package chclient

import (
	"context"
	"fmt"
	"time"
)

// HeartbeatTimeout contains an agent with a missing or stale heartbeat.
type HeartbeatTimeout struct {
	AgentID       string    `ch:"agent_id"`
	LastHeartbeat time.Time `ch:"last_heartbeat"`
}

// GetRecentTraces returns the most recent traces, optionally filtered by agentID.
// When agentID is empty, all traces are returned.
func (c *Client) GetRecentTraces(ctx context.Context, agentID string, limit int) ([]AuditTrace, error) {
	var (
		query string
		args  []any
	)
	if agentID == "" {
		query = `
SELECT trace_id, span_id, agent_id, agent_role, request_type,
       method, path, status_code, request_body, response_body,
       tool_calls, model, tokens_in, tokens_out, latency_ms, created_at
FROM audit.traces
ORDER BY created_at DESC
LIMIT ?`
		args = []any{limit}
	} else {
		query = `
SELECT trace_id, span_id, agent_id, agent_role, request_type,
       method, path, status_code, request_body, response_body,
       tool_calls, model, tokens_in, tokens_out, latency_ms, created_at
FROM audit.traces
WHERE agent_id = ?
ORDER BY created_at DESC
LIMIT ?`
		args = []any{agentID, limit}
	}

	rows, err := c.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query traces: %w", err)
	}
	defer rows.Close()

	var traces []AuditTrace
	for rows.Next() {
		var t AuditTrace
		if err := rows.ScanStruct(&t); err != nil {
			return nil, fmt.Errorf("scan trace: %w", err)
		}
		traces = append(traces, t)
	}
	return traces, rows.Err()
}

// GetHeartbeatTimeouts returns agents whose last heartbeat is older than the timeout duration.
// Used by Backend for heartbeat monitoring (not by Operator directly).
func (c *Client) GetHeartbeatTimeouts(ctx context.Context, timeout time.Duration) ([]HeartbeatTimeout, error) {
	query := `
SELECT agent_id, max(created_at) AS last_heartbeat
FROM audit.traces
WHERE request_type = 'heartbeat'
  AND created_at >= now() - INTERVAL 1 HOUR
GROUP BY agent_id
HAVING last_heartbeat < now() - INTERVAL ? SECOND`

	rows, err := c.conn.Query(ctx, query, int64(timeout.Seconds()))
	if err != nil {
		return nil, fmt.Errorf("query heartbeat timeouts: %w", err)
	}
	defer rows.Close()

	var timeouts []HeartbeatTimeout
	for rows.Next() {
		var h HeartbeatTimeout
		if err := rows.Scan(&h.AgentID, &h.LastHeartbeat); err != nil {
			return nil, fmt.Errorf("scan heartbeat timeout: %w", err)
		}
		timeouts = append(timeouts, h)
	}
	return timeouts, rows.Err()
}
