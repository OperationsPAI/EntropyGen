package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/entropyGen/entropyGen/internal/common/models"
	"github.com/entropyGen/entropyGen/internal/common/redisclient"
)

const (
	streamGitea     = "events:gitea"
	streamMaxLen    = int64(10000)
	pushMaxBytes    = 4 * 1024  // 4KB: strip commits[].diff if exceeded
	defaultMaxBytes = 16 * 1024 // 16KB: truncate whole payload if exceeded
)

// GiteaHandler handles POST /webhook/gitea from Gitea.
type GiteaHandler struct {
	secret []byte
	writer *redisclient.StreamWriter
}

// NewGiteaHandler creates a GiteaHandler. secret is the HMAC-SHA256 signing secret.
func NewGiteaHandler(secret string, writer *redisclient.StreamWriter) *GiteaHandler {
	return &GiteaHandler{secret: []byte(secret), writer: writer}
}

func (h *GiteaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if !h.verifySignature(body, r.Header.Get("X-Gitea-Signature")) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	giteaEvent := r.Header.Get("X-Gitea-Event")
	event, err := h.parseEvent(giteaEvent, body)
	if err != nil {
		slog.Warn("webhook: parse event failed", "event", giteaEvent, "err", err)
		w.WriteHeader(http.StatusOK) // still return 200 to avoid Gitea retry
		return
	}
	if event == nil {
		w.WriteHeader(http.StatusOK) // unknown event type — ignore silently
		return
	}

	// Write to Redis asynchronously so we return 200 immediately
	go func() {
		if err := h.writer.Write(r.Context(), streamGitea, event, streamMaxLen); err != nil {
			slog.Warn("webhook: redis write failed", "event_type", event.EventType, "err", err)
		}
	}()

	w.WriteHeader(http.StatusOK)
}

func (h *GiteaHandler) verifySignature(body []byte, sigHeader string) bool {
	if len(h.secret) == 0 {
		return true
	}
	mac := hmac.New(sha256.New, h.secret)
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	sig := strings.TrimPrefix(sigHeader, "sha256=")
	return hmac.Equal([]byte(expected), []byte(sig))
}

func (h *GiteaHandler) parseEvent(giteaEvent string, body []byte) (*models.Event, error) {
	now := time.Now()
	id := uuid.New().String()

	switch giteaEvent {
	case "push":
		payload, err := truncatePushPayload(body)
		if err != nil {
			return nil, err
		}
		return &models.Event{EventID: id, EventType: models.EventTypeGiteaPush, Timestamp: now, Payload: payload}, nil

	case "issues":
		var raw struct {
			Action string `json:"action"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, err
		}
		var et string
		switch raw.Action {
		case "opened":
			et = models.EventTypeGiteaIssueOpen
		case "closed":
			et = "gitea.issue_close"
		default:
			return nil, nil
		}
		return &models.Event{EventID: id, EventType: et, Timestamp: now, Payload: truncatePayload(body, defaultMaxBytes)}, nil

	case "issue_comment":
		return &models.Event{EventID: id, EventType: models.EventTypeGiteaIssueComment, Timestamp: now, Payload: truncatePayload(body, defaultMaxBytes)}, nil

	case "pull_request":
		var raw struct {
			Action      string `json:"action"`
			PullRequest struct {
				Merged bool `json:"merged"`
			} `json:"pull_request"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, err
		}
		var et string
		switch {
		case raw.Action == "closed" && raw.PullRequest.Merged:
			et = models.EventTypeGiteaPRMerge
		case raw.Action == "opened":
			et = models.EventTypeGiteaPROpen
		default:
			return nil, nil
		}
		return &models.Event{EventID: id, EventType: et, Timestamp: now, Payload: truncatePayload(body, defaultMaxBytes)}, nil

	case "pull_request_review_comment":
		return &models.Event{EventID: id, EventType: models.EventTypeGiteaPRReview, Timestamp: now, Payload: truncatePayload(body, defaultMaxBytes)}, nil

	case "workflow_run":
		return &models.Event{EventID: id, EventType: models.EventTypeGiteaCIStatus, Timestamp: now, Payload: truncatePayload(body, defaultMaxBytes)}, nil

	default:
		return nil, nil
	}
}

// truncatePushPayload strips commits[].diff if total body > pushMaxBytes.
func truncatePushPayload(body []byte) (json.RawMessage, error) {
	if len(body) <= pushMaxBytes {
		return json.RawMessage(body), nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return truncatePayload(body, pushMaxBytes), nil
	}
	var commits []map[string]json.RawMessage
	if err := json.Unmarshal(raw["commits"], &commits); err == nil {
		for i := range commits {
			delete(commits[i], "diff")
		}
		stripped, _ := json.Marshal(commits)
		raw["commits"] = json.RawMessage(stripped)
	}
	result, _ := json.Marshal(raw)
	return json.RawMessage(result), nil
}

// truncatePayload returns body unchanged if within maxBytes,
// otherwise returns a JSON object with _truncated marker.
func truncatePayload(body []byte, maxBytes int) json.RawMessage {
	if len(body) <= maxBytes {
		return json.RawMessage(body)
	}
	return json.RawMessage(fmt.Sprintf(`{"_truncated":true,"_original_size":%d}`, len(body)))
}
