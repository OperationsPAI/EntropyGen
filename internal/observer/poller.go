package observer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/entropyGen/entropyGen/internal/common/models"
	"github.com/entropyGen/entropyGen/internal/common/redisclient"
)

const (
	pollBatch = 10
	pollBlock = 5 * time.Second
)

// Poller reads cron.trigger events from a Redis Stream and forwards them
// to the openclaw gateway's chat completions endpoint.
type Poller struct {
	reader        *redisclient.StreamReader
	stream        string
	openclawURL   string
	openclawToken string
	httpClient    *http.Client
}

// NewPoller creates a Poller that reads from the given stream and sends
// prompts to the openclaw gateway at openclawURL.
func NewPoller(reader *redisclient.StreamReader, stream, openclawURL, openclawToken string) *Poller {
	return &Poller{
		reader:        reader,
		stream:        stream,
		openclawURL:   openclawURL,
		openclawToken: openclawToken,
		httpClient:    &http.Client{Timeout: 5 * time.Minute},
	}
}

// Run starts the polling loop. It blocks until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) error {
	if err := p.reader.CreateGroup(ctx, p.stream); err != nil {
		return fmt.Errorf("create consumer group for %s: %w", p.stream, err)
	}

	slog.Info("poller started", "stream", p.stream)

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		msgs, err := p.reader.Read(ctx, p.stream, pollBatch, pollBlock)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Error("poller read error", "stream", p.stream, "err", err)
			time.Sleep(2 * time.Second)
			continue
		}

		for _, msg := range msgs {
			if msg.Event.EventType != models.EventTypeCronTrigger {
				_ = p.reader.ACK(ctx, p.stream, msg.ID)
				continue
			}

			var payload models.CronTriggerPayload
			if err := json.Unmarshal(msg.Event.Payload, &payload); err != nil {
				slog.Error("unmarshal cron payload", "id", msg.ID, "err", err)
				_ = p.reader.ACK(ctx, p.stream, msg.ID)
				continue
			}

			if err := p.sendToOpenClaw(ctx, payload.Prompt); err != nil {
				slog.Error("send to openclaw failed", "id", msg.ID, "err", err)
				// Don't ACK — retry on next poll
				continue
			}

			_ = p.reader.ACK(ctx, p.stream, msg.ID)
			slog.Info("cron trigger processed", "id", msg.ID, "agent", msg.Event.AgentID)
		}
	}
}

func (p *Poller) sendToOpenClaw(ctx context.Context, prompt string) error {
	body := map[string]interface{}{
		"model": "openclaw:main",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream": false,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/chat/completions", p.openclawURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.openclawToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("openclaw returned status %d", resp.StatusCode)
	}
	return nil
}
