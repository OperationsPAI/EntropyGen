package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/entropyGen/entropyGen/internal/common/models"
	"github.com/entropyGen/entropyGen/internal/common/redisclient"
)

const (
	channelSize  = 1000
	maxBodyBytes = 64 * 1024 // 64KB
	streamName   = "events:gateway"
	streamMaxLen = int64(100000)
)

// RequestRecord holds the data extracted from a proxied HTTP request/response.
type RequestRecord struct {
	TraceID   string
	EventType string
	AgentID   string
	AgentRole string
	Method    string
	Path      string
	Status    int
	LatencyMs int64
	ReqBody   []byte
	RespBody  []byte
	IsLLM     bool
}

// EventWriter asynchronously writes audit events to Redis Stream events:gateway.
// If the internal channel is full, events are dropped with a warning log (never blocks callers).
// If Redis is unreachable, events are logged as warnings (never returns errors to callers).
type EventWriter struct {
	ch     chan *models.Event
	writer *redisclient.StreamWriter // nil for nop writer
}

// NewEventWriter creates an EventWriter backed by the given Redis StreamWriter.
func NewEventWriter(writer *redisclient.StreamWriter) *EventWriter {
	return &EventWriter{
		ch:     make(chan *models.Event, channelSize),
		writer: writer,
	}
}

// NewNopEventWriter creates an EventWriter that discards all events (useful for tests).
func NewNopEventWriter() *EventWriter {
	return &EventWriter{ch: make(chan *models.Event, channelSize)}
}

// Enqueue adds a pre-built event to the write queue (non-blocking, drops if full).
func (ew *EventWriter) Enqueue(event *models.Event) {
	select {
	case ew.ch <- event:
	default:
		slog.Warn("audit channel full, dropping event",
			"event_type", event.EventType,
			"agent_id", event.AgentID,
		)
	}
}

// EnqueueRequest builds an event from a RequestRecord and enqueues it.
func (ew *EventWriter) EnqueueRequest(rec RequestRecord) {
	payload := buildPayload(rec)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("failed to marshal audit payload", "err", err)
		return
	}

	ew.Enqueue(&models.Event{
		EventID:   uuid.New().String(),
		EventType: rec.EventType,
		Timestamp: time.Now(),
		AgentID:   rec.AgentID,
		AgentRole: rec.AgentRole,
		Payload:   payloadBytes,
	})
}

// Run processes the event queue and writes to Redis. Call this in a goroutine.
// Returns when ctx is cancelled.
func (ew *EventWriter) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ew.ch:
			if !ok {
				return
			}
			if ew.writer == nil {
				continue // nop: discard
			}
			if err := ew.writer.Write(ctx, streamName, event, streamMaxLen); err != nil {
				slog.Warn("audit write to Redis failed, degrading to log",
					"event_type", event.EventType,
					"agent_id", event.AgentID,
					"err", err,
				)
			}
		}
	}
}

// buildPayload constructs the event payload map from a RequestRecord.
func buildPayload(rec RequestRecord) map[string]interface{} {
	p := map[string]interface{}{
		"trace_id":    rec.TraceID,
		"method":      rec.Method,
		"path":        rec.Path,
		"status_code": rec.Status,
		"latency_ms":  rec.LatencyMs,
	}
	if rec.IsLLM {
		reqStr, reqTrunc := truncateBody(rec.ReqBody)
		respStr, respTrunc := truncateBody(rec.RespBody)
		p["request_body"] = reqStr
		p["response_body"] = respStr
		p["_body_truncated"] = reqTrunc || respTrunc
	} else {
		p["request_body"] = ""
		p["response_body"] = ""
		p["_body_truncated"] = false
	}
	return p
}

// truncateBody returns the body as a string, truncated to maxBodyBytes if needed.
// Returns (body string, was_truncated).
func truncateBody(body []byte) (string, bool) {
	if len(body) == 0 {
		return "", false
	}
	if len(body) > maxBodyBytes {
		return string(body[:maxBodyBytes]) + "...[TRUNCATED]", true
	}
	return string(body), false
}
