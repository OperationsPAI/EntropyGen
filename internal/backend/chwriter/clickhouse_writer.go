package chwriter

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/entropyGen/entropyGen/internal/common/chclient"
	"github.com/entropyGen/entropyGen/internal/common/models"
	"github.com/entropyGen/entropyGen/internal/common/redisclient"
)

const (
	batchSize    = 100
	batchTimeout = 5 * time.Second
	maxRetries   = 3
)

var watchedStreams = []string{"events:gateway", "events:gitea", "events:k8s"}

// Writer consumes events from Redis Streams and batch-writes them to ClickHouse.
type Writer struct {
	ch           *chclient.Client
	reader       *redisclient.StreamReader
	streamWriter *redisclient.StreamWriter
	dlqDir       string
}

// New creates a new ClickHouse Writer.
func New(ch *chclient.Client, reader *redisclient.StreamReader, sw *redisclient.StreamWriter, dlqDir string) *Writer {
	return &Writer{ch: ch, reader: reader, streamWriter: sw, dlqDir: dlqDir}
}

// Run starts the consumer loop. Blocks until ctx is cancelled.
func (w *Writer) Run(ctx context.Context) {
	// Process pending messages from a previous run
	w.reclaimPending(ctx)

	ticker := time.NewTicker(batchTimeout)
	defer ticker.Stop()

	buffers := make(map[string][]redisclient.StreamMessage)
	for _, s := range watchedStreams {
		buffers[s] = make([]redisclient.StreamMessage, 0, batchSize)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, stream := range watchedStreams {
				if len(buffers[stream]) > 0 {
					w.flush(ctx, stream, buffers[stream])
					buffers[stream] = buffers[stream][:0]
				}
			}
		default:
			for _, stream := range watchedStreams {
				msgs, err := w.reader.Read(ctx, stream, int64(batchSize), 200*time.Millisecond)
				if err != nil {
					slog.Warn("chwriter: read failed", "stream", stream, "err", err)
					continue
				}
				buffers[stream] = append(buffers[stream], msgs...)
				if len(buffers[stream]) >= batchSize {
					w.flush(ctx, stream, buffers[stream])
					buffers[stream] = buffers[stream][:0]
				}
			}
		}
	}
}

func (w *Writer) flush(ctx context.Context, stream string, msgs []redisclient.StreamMessage) {
	if len(msgs) == 0 {
		return
	}
	traces := make([]chclient.AuditTrace, 0, len(msgs))
	ids := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		ids = append(ids, msg.ID)
		if msg.Event != nil {
			traces = append(traces, EventToTrace(msg.Event))
		}
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := w.ch.InsertTraces(ctx, traces); err != nil {
			lastErr = err
			slog.Warn("chwriter: insert attempt failed",
				"stream", stream, "attempt", attempt+1, "err", err)
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		_ = w.reader.ACK(ctx, stream, ids...)
		return
	}

	// All retries failed: write to DLQ then ACK to unblock
	slog.Error("chwriter: all retries failed, writing DLQ",
		"stream", stream, "count", len(msgs), "err", lastErr)
	w.writeDLQ(stream, msgs)
	_ = w.reader.ACK(ctx, stream, ids...)
}

func (w *Writer) reclaimPending(ctx context.Context) {
	for _, stream := range watchedStreams {
		msgs, err := w.reader.Claim(ctx, stream, 30*time.Second, "0-0", 100)
		if err != nil {
			continue
		}
		if len(msgs) > 0 {
			slog.Info("chwriter: reprocessing pending", "stream", stream, "count", len(msgs))
			w.flush(ctx, stream, msgs)
		}
	}
}

func (w *Writer) writeDLQ(stream string, msgs []redisclient.StreamMessage) {
	if err := os.MkdirAll(w.dlqDir, 0o755); err != nil {
		slog.Error("chwriter: DLQ mkdir failed", "err", err)
		return
	}
	fname := filepath.Join(w.dlqDir,
		"dlq-"+time.Now().Format("20060102-150405")+"-"+stream+".jsonl")
	fname = filepath.Clean(fname)
	f, err := os.Create(fname)
	if err != nil {
		slog.Error("chwriter: DLQ create failed", "err", err)
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, msg := range msgs {
		if msg.Event != nil {
			_ = enc.Encode(msg.Event)
		}
	}
	slog.Info("chwriter: DLQ file written", "file", fname, "count", len(msgs))
}

// EventToTrace converts a models.Event to an AuditTrace row.
// Exported for testing.
func EventToTrace(e *models.Event) chclient.AuditTrace {
	t := chclient.NewAuditTrace()
	t.AgentID = e.AgentID
	t.AgentRole = e.AgentRole
	t.CreatedAt = e.Timestamp

	// request_type = event_type with prefix stripped
	switch {
	case len(e.EventType) > 8 && e.EventType[:8] == "gateway.":
		t.RequestType = e.EventType[8:]
	case len(e.EventType) > 6 && e.EventType[:6] == "gitea.":
		t.RequestType = e.EventType[6:]
	case len(e.EventType) > 4 && e.EventType[:4] == "k8s.":
		t.RequestType = e.EventType[4:]
	default:
		t.RequestType = e.EventType
	}

	var payload map[string]json.RawMessage
	if e.Payload == nil || json.Unmarshal(e.Payload, &payload) != nil {
		return t
	}

	getString := func(key string) string {
		if v, ok := payload[key]; ok {
			var s string
			if json.Unmarshal(v, &s) == nil {
				return s
			}
		}
		return ""
	}
	getUint32 := func(key string) uint32 {
		if v, ok := payload[key]; ok {
			var n uint32
			if json.Unmarshal(v, &n) == nil {
				return n
			}
		}
		return 0
	}

	t.Method = getString("method")
	t.Path = getString("path")
	t.Model = getString("model")
	t.RequestBody = getString("request_body")
	t.ResponseBody = getString("response_body")
	t.TokensIn = getUint32("tokens_in")
	t.TokensOut = getUint32("tokens_out")
	t.LatencyMs = getUint32("latency_ms")
	if sc := getUint32("status_code"); sc > 0 {
		t.StatusCode = uint16(sc)
	}
	return t
}
