package chclient

import (
	"context"
	"fmt"
	"time"
)

// agentOnlyFilter is a WHERE clause fragment that restricts results to real agent pods,
// excluding infrastructure components (gateway, operator, backend, frontend, etc.).
const agentOnlyFilter = `agent_id LIKE 'agent-%'
  AND agent_id NOT IN ('agent-gateway')`

// HeartbeatTimeout contains an agent with a missing or stale heartbeat.
type HeartbeatTimeout struct {
	AgentID       string    `ch:"agent_id"`
	LastHeartbeat time.Time `ch:"last_heartbeat"`
}

// TraceFilter holds filtering/pagination parameters for trace queries.
type TraceFilter struct {
	AgentID     string
	RequestType string
	Status      string // "success" or "error"
	StartTime   string // YYYY-MM-DD
	EndTime     string // YYYY-MM-DD
	Limit       int
	Page        int
}

// TraceResult holds paginated trace results.
type TraceResult struct {
	Items []AuditTrace `json:"items"`
	Total int          `json:"total"`
}

// GetRecentTraces returns the most recent traces, optionally filtered by agentID.
// When agentID is empty, all traces are returned.
func (c *Client) GetRecentTraces(ctx context.Context, agentID string, limit int) ([]AuditTrace, error) {
	f := TraceFilter{AgentID: agentID, Limit: limit, Page: 1}
	result, err := c.QueryTraces(ctx, f)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// QueryTraces returns traces matching the given filter with pagination.
func (c *Client) QueryTraces(ctx context.Context, f TraceFilter) (*TraceResult, error) {
	if f.Limit < 1 {
		f.Limit = 50
	}
	if f.Limit > 200 {
		f.Limit = 200
	}
	if f.Page < 1 {
		f.Page = 1
	}
	offset := (f.Page - 1) * f.Limit

	// Build WHERE clauses
	var conditions []string
	var args []any

	if f.AgentID != "" {
		conditions = append(conditions, "agent_id = ?")
		args = append(args, f.AgentID)
	}
	if f.RequestType != "" {
		conditions = append(conditions, "request_type = ?")
		args = append(args, f.RequestType)
	}
	if f.Status == "success" {
		conditions = append(conditions, "status_code < 400")
	} else if f.Status == "error" {
		conditions = append(conditions, "status_code >= 400")
	}
	if f.StartTime != "" {
		conditions = append(conditions, "created_at >= toDate(?)")
		args = append(args, f.StartTime)
	}
	if f.EndTime != "" {
		conditions = append(conditions, "created_at < toDate(?) + INTERVAL 1 DAY")
		args = append(args, f.EndTime)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + conditions[0]
		for _, cond := range conditions[1:] {
			where += " AND " + cond
		}
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT count() FROM audit.traces %s", where)
	var total uint64
	if err := c.conn.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count traces: %w", err)
	}

	// Fetch page
	dataQuery := fmt.Sprintf(`
SELECT trace_id, span_id, agent_id, agent_role, request_type,
       method, path, status_code, request_body, response_body,
       tool_calls, model, tokens_in, tokens_out, latency_ms, created_at
FROM audit.traces
%s
ORDER BY created_at DESC
LIMIT ? OFFSET ?`, where)
	dataArgs := append(args, f.Limit, offset)

	rows, err := c.conn.Query(ctx, dataQuery, dataArgs...)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &TraceResult{Items: traces, Total: int(total)}, nil
}

// TokenUsageSummary holds daily token usage for an agent.
type TokenUsageSummary struct {
	AgentID      string    `ch:"agent_id" json:"agent_id"`
	Date         time.Time `ch:"date" json:"date"`
	TokensIn     uint64    `ch:"tokens_in" json:"tokens_in"`
	TokensOut    uint64    `ch:"tokens_out" json:"tokens_out"`
	RequestCount uint64    `ch:"request_count" json:"request_count"`
	Model        string    `ch:"model" json:"model"`
}

// GetTokenUsage returns daily token usage grouped by agent and model.
func (c *Client) GetTokenUsage(ctx context.Context, agentID string, days int) ([]TokenUsageSummary, error) {
	if days < 1 {
		days = 30
	}

	var where string
	var args []any
	if agentID != "" {
		where = "AND agent_id = ?"
		args = append(args, agentID)
	}

	query := fmt.Sprintf(`
SELECT agent_id,
       toDate(created_at) AS date,
       sum(tokens_in) AS tokens_in,
       sum(tokens_out) AS tokens_out,
       count() AS request_count,
       if(model = '', 'unknown', model) AS model
FROM audit.traces
WHERE created_at >= today() - INTERVAL ? DAY
  AND %s
  %s
GROUP BY agent_id, date, model
ORDER BY date DESC, agent_id`, agentOnlyFilter, where)
	args = append([]any{days}, args...)

	rows, err := c.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query token usage: %w", err)
	}
	defer rows.Close()

	var results []TokenUsageSummary
	for rows.Next() {
		var s TokenUsageSummary
		if err := rows.Scan(&s.AgentID, &s.Date, &s.TokensIn, &s.TokensOut, &s.RequestCount, &s.Model); err != nil {
			return nil, fmt.Errorf("scan token usage: %w", err)
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

// AgentActivitySummary holds hourly activity counts for an agent.
type AgentActivitySummary struct {
	AgentID string `json:"agent_id"`
	Hour    uint8  `json:"hour"`
	Count   uint64 `json:"count"`
}

// GetAgentActivity returns hourly request counts grouped by agent.
func (c *Client) GetAgentActivity(ctx context.Context, agentID string, days int) ([]AgentActivitySummary, error) {
	if days < 1 {
		days = 7
	}

	var where string
	var args []any
	if agentID != "" {
		where = "AND agent_id = ?"
		args = append(args, agentID)
	}

	query := fmt.Sprintf(`
SELECT agent_id,
       toHour(created_at) AS hour,
       count() AS count
FROM audit.traces
WHERE created_at >= today() - INTERVAL ? DAY
  AND %s
  %s
GROUP BY agent_id, hour
ORDER BY agent_id, hour`, agentOnlyFilter, where)
	args = append([]any{days}, args...)

	rows, err := c.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query agent activity: %w", err)
	}
	defer rows.Close()

	var results []AgentActivitySummary
	for rows.Next() {
		var s AgentActivitySummary
		if err := rows.Scan(&s.AgentID, &s.Hour, &s.Count); err != nil {
			return nil, fmt.Errorf("scan agent activity: %w", err)
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

// ModelDistribution holds request counts grouped by model.
type ModelDistribution struct {
	Model string `json:"model"`
	Count uint64 `json:"count"`
}

// GetModelDistribution returns request counts grouped by model.
func (c *Client) GetModelDistribution(ctx context.Context, days int) ([]ModelDistribution, error) {
	if days < 1 {
		days = 30
	}

	query := fmt.Sprintf(`
SELECT if(model = '', 'unknown', model) AS model,
       count() AS count
FROM audit.traces
WHERE created_at >= today() - INTERVAL ? DAY
  AND %s
  AND request_type NOT IN ('heartbeat', 'pod_status')
GROUP BY model
ORDER BY count DESC`, agentOnlyFilter)

	rows, err := c.conn.Query(ctx, query, days)
	if err != nil {
		return nil, fmt.Errorf("query model distribution: %w", err)
	}
	defer rows.Close()

	var results []ModelDistribution
	for rows.Next() {
		var d ModelDistribution
		if err := rows.Scan(&d.Model, &d.Count); err != nil {
			return nil, fmt.Errorf("scan model distribution: %w", err)
		}
		results = append(results, d)
	}
	return results, rows.Err()
}

// LatencyPoint holds average latency for a date.
type LatencyPoint struct {
	Date  time.Time `json:"date"`
	AvgMs float64   `json:"avg_ms"`
	P95Ms float64   `json:"p95_ms"`
}

// GetLatencyTrend returns daily average and p95 latency.
func (c *Client) GetLatencyTrend(ctx context.Context, days int) ([]LatencyPoint, error) {
	if days < 1 {
		days = 30
	}

	query := fmt.Sprintf(`
SELECT toDate(created_at) AS date,
       avg(latency_ms) AS avg_ms,
       quantile(0.95)(latency_ms) AS p95_ms
FROM audit.traces
WHERE created_at >= today() - INTERVAL ? DAY
  AND %s
  AND request_type NOT IN ('heartbeat', 'pod_status')
  AND latency_ms > 0
GROUP BY date
ORDER BY date`, agentOnlyFilter)

	rows, err := c.conn.Query(ctx, query, days)
	if err != nil {
		return nil, fmt.Errorf("query latency trend: %w", err)
	}
	defer rows.Close()

	var results []LatencyPoint
	for rows.Next() {
		var p LatencyPoint
		if err := rows.Scan(&p.Date, &p.AvgMs, &p.P95Ms); err != nil {
			return nil, fmt.Errorf("scan latency: %w", err)
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

// AgentRanking holds total token usage for an agent.
type AgentRanking struct {
	AgentID    string `json:"agent_id"`
	TotalUsage uint64 `json:"total_usage"`
}

// GetAgentRanking returns agents ranked by request count today.
func (c *Client) GetAgentRanking(ctx context.Context) ([]AgentRanking, error) {
	query := fmt.Sprintf(`
SELECT agent_id,
       count() AS total_usage
FROM audit.traces
WHERE created_at >= today()
  AND %s
GROUP BY agent_id
ORDER BY total_usage DESC`, agentOnlyFilter)

	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query agent ranking: %w", err)
	}
	defer rows.Close()

	var results []AgentRanking
	for rows.Next() {
		var r AgentRanking
		if err := rows.Scan(&r.AgentID, &r.TotalUsage); err != nil {
			return nil, fmt.Errorf("scan agent ranking: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
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

// AgentSummary contains per-agent stats for display in agent list.
type AgentSummary struct {
	AgentID         string    `json:"agent_id"`
	TodayTokens     uint64    `json:"today_tokens"`
	TotalTokens     uint64    `json:"total_tokens"`
	LastDescription string    `json:"last_description"`
	LastTimestamp    time.Time `json:"last_timestamp"`
}

// GetAgentSummaries returns per-agent token counts (today and total) and last meaningful action.
func (c *Client) GetAgentSummaries(ctx context.Context) (map[string]*AgentSummary, error) {
	// Token usage per agent
	tokenQuery := `
SELECT agent_id,
       sumIf(tokens_in + tokens_out, created_at >= today()) AS today_tokens,
       sum(tokens_in + tokens_out) AS total_tokens
FROM audit.traces
GROUP BY agent_id`

	rows, err := c.conn.Query(ctx, tokenQuery)
	if err != nil {
		return nil, fmt.Errorf("query agent token summaries: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*AgentSummary)
	for rows.Next() {
		var s AgentSummary
		if err := rows.Scan(&s.AgentID, &s.TodayTokens, &s.TotalTokens); err != nil {
			return nil, fmt.Errorf("scan agent token summary: %w", err)
		}
		result[s.AgentID] = &s
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Last action per agent (most recent trace, preferring non-heartbeat/non-pod_status)
	lastActionQuery := `
SELECT agent_id,
       request_type || ' ' || method || ' ' || path AS description,
       created_at
FROM audit.traces
WHERE (agent_id, created_at) IN (
    SELECT agent_id, max(created_at)
    FROM audit.traces
    GROUP BY agent_id
)
ORDER BY agent_id`

	rows2, err := c.conn.Query(ctx, lastActionQuery)
	if err != nil {
		// Non-fatal: just skip last action data
		return result, nil
	}
	defer rows2.Close()

	for rows2.Next() {
		var agentID, desc string
		var ts time.Time
		if err := rows2.Scan(&agentID, &desc, &ts); err != nil {
			continue
		}
		s, ok := result[agentID]
		if !ok {
			s = &AgentSummary{AgentID: agentID}
			result[agentID] = s
		}
		s.LastDescription = desc
		s.LastTimestamp = ts
	}

	return result, nil
}
