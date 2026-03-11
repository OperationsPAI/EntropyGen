package chclient

import (
	"context"
	"fmt"
)

// CreateSchema creates the audit database, traces table, and materialized views.
// Idempotent: uses IF NOT EXISTS on all DDL statements.
func (c *Client) CreateSchema(ctx context.Context) error {
	ddls := []string{
		`CREATE DATABASE IF NOT EXISTS audit`,

		`CREATE TABLE IF NOT EXISTS audit.traces (
    trace_id     UUID,
    span_id      UUID,
    agent_id     String,
    agent_role   LowCardinality(String),
    request_type LowCardinality(String),
    method       LowCardinality(String),
    path         String,
    status_code  UInt16,
    request_body  String,
    response_body String,
    tool_calls    String,
    model         LowCardinality(String),
    tokens_in     UInt32 DEFAULT 0,
    tokens_out    UInt32 DEFAULT 0,
    latency_ms    UInt32,
    created_at    DateTime64(3) DEFAULT now64(3)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (agent_id, request_type, created_at)
TTL toDateTime(created_at) + INTERVAL 90 DAY
SETTINGS index_granularity = 8192`,

		`CREATE TABLE IF NOT EXISTS audit.token_usage_daily (
    day           Date,
    agent_id      String,
    agent_role    LowCardinality(String),
    model         LowCardinality(String),
    total_tokens_in  UInt64,
    total_tokens_out UInt64,
    request_count    UInt64,
    avg_latency_ms   Float64
) ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(day)
ORDER BY (day, agent_id, model)`,

		`CREATE MATERIALIZED VIEW IF NOT EXISTS audit.token_usage_daily_mv
TO audit.token_usage_daily
AS SELECT
    toDate(created_at) AS day,
    agent_id,
    agent_role,
    model,
    sum(tokens_in) AS total_tokens_in,
    sum(tokens_out) AS total_tokens_out,
    count() AS request_count,
    avg(latency_ms) AS avg_latency_ms
FROM audit.traces
WHERE request_type = 'llm_inference'
GROUP BY day, agent_id, agent_role, model`,

		`CREATE TABLE IF NOT EXISTS audit.agent_operations_hourly (
    hour             DateTime,
    agent_id         String,
    agent_role       LowCardinality(String),
    operation_type   LowCardinality(String),
    path             String,
    operation_count  UInt64
) ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (hour, agent_id, operation_type)`,

		`CREATE MATERIALIZED VIEW IF NOT EXISTS audit.agent_operations_hourly_mv
TO audit.agent_operations_hourly
AS SELECT
    toStartOfHour(created_at) AS hour,
    agent_id,
    agent_role,
    request_type AS operation_type,
    path,
    count() AS operation_count
FROM audit.traces
WHERE request_type = 'gitea_api'
GROUP BY hour, agent_id, agent_role, operation_type, path`,
	}

	for _, ddl := range ddls {
		if err := c.conn.Exec(ctx, ddl); err != nil {
			truncLen := len(ddl)
			if truncLen > 100 {
				truncLen = 100
			}
			return fmt.Errorf("DDL exec failed: %w\nSQL: %s", err, ddl[:truncLen])
		}
	}
	return nil
}
