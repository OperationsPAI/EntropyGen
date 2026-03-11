package webhook_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/entropyGen/entropyGen/internal/common/redisclient"
	"github.com/entropyGen/entropyGen/internal/event-collector/webhook"
)

const testSecret = "test-webhook-secret"

func sign(body []byte) string {
	mac := hmac.New(sha256.New, []byte(testSecret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func newTestHandler(t *testing.T) (*webhook.GiteaHandler, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return webhook.NewGiteaHandler(testSecret, redisclient.NewStreamWriter(rdb)), rdb
}

// waitForStream polls until the stream has at least one entry or times out.
func waitForStream(rdb *redis.Client, stream string, timeout time.Duration) bool {
	ctx := context.Background()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		n, err := rdb.XLen(ctx, stream).Result()
		if err == nil && n > 0 {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// streamLen returns the current length of a Redis stream.
func streamLen(rdb *redis.Client, stream string) int64 {
	n, _ := rdb.XLen(context.Background(), stream).Result()
	return n
}

func TestGiteaHandler_IssueOpen_ValidSignature(t *testing.T) {
	h, rdb := newTestHandler(t)
	body := []byte(`{"action":"opened","issue":{"number":1,"title":"Test"}}`)
	req := httptest.NewRequest("POST", "/webhook/gitea", bytes.NewReader(body))
	req.Header.Set("X-Gitea-Event", "issues")
	req.Header.Set("X-Gitea-Signature", sign(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if !waitForStream(rdb, "events:gitea", time.Second) {
		t.Error("expected event in events:gitea stream")
	}
}

func TestGiteaHandler_InvalidSignature(t *testing.T) {
	h, rdb := newTestHandler(t)
	body := []byte(`{"action":"opened"}`)
	req := httptest.NewRequest("POST", "/webhook/gitea", bytes.NewReader(body))
	req.Header.Set("X-Gitea-Event", "issues")
	req.Header.Set("X-Gitea-Signature", "badsignature")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
	time.Sleep(50 * time.Millisecond)
	if streamLen(rdb, "events:gitea") != 0 {
		t.Error("invalid signature should not write to Redis")
	}
}

func TestGiteaHandler_PRMerge(t *testing.T) {
	h, rdb := newTestHandler(t)
	body := []byte(`{"action":"closed","pull_request":{"number":5,"title":"Fix","merged":true}}`)
	req := httptest.NewRequest("POST", "/webhook/gitea", bytes.NewReader(body))
	req.Header.Set("X-Gitea-Event", "pull_request")
	req.Header.Set("X-Gitea-Signature", sign(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if !waitForStream(rdb, "events:gitea", time.Second) {
		t.Error("expected PR merge event in stream")
	}
}

func TestGiteaHandler_PushTruncation(t *testing.T) {
	commits := make([]map[string]interface{}, 3)
	for i := range commits {
		commits[i] = map[string]interface{}{
			"id":      "abc123",
			"message": "commit",
			"author":  map[string]string{"name": "agent"},
			"diff":    string(make([]byte, 2000)),
		}
	}
	payload := map[string]interface{}{"ref": "refs/heads/main", "commits": commits}
	body, _ := json.Marshal(payload)
	if len(body) <= 4*1024 {
		t.Skip("body not large enough to trigger truncation")
	}

	h, _ := newTestHandler(t)
	req := httptest.NewRequest("POST", "/webhook/gitea", bytes.NewReader(body))
	req.Header.Set("X-Gitea-Event", "push")
	req.Header.Set("X-Gitea-Signature", sign(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("push truncation: status got %d, want 200", rec.Code)
	}
}

func TestGiteaHandler_UnknownEvent_Returns200(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{}`)
	req := httptest.NewRequest("POST", "/webhook/gitea", bytes.NewReader(body))
	req.Header.Set("X-Gitea-Event", "unknown_event_type")
	req.Header.Set("X-Gitea-Signature", sign(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("unknown event: status got %d, want 200", rec.Code)
	}
}
