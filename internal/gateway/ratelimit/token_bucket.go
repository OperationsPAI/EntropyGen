package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"github.com/entropyGen/entropyGen/internal/gateway/gatewayctx"
)

// bucket implements a token bucket for a single agent.
type bucket struct {
	mu           sync.Mutex
	tokens       float64
	maxTokens    float64
	refillPerSec float64
	lastRefill   time.Time
}

func newBucket(rpm, burst int) *bucket {
	return &bucket{
		tokens:       float64(burst),
		maxTokens:    float64(burst),
		refillPerSec: float64(rpm) / 60.0,
		lastRefill:   time.Now(),
	}
}

func (b *bucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.refillPerSec
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastRefill = now

	if b.tokens < 1.0 {
		return false
	}
	b.tokens--
	return true
}

func (b *bucket) lastActive() time.Time {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastRefill
}

// Limiter manages per-agent token buckets using a sync.Map for concurrent access.
type Limiter struct {
	buckets sync.Map
	rpm     int
	burst   int
}

// NewLimiter creates a Limiter with the given requests-per-minute and burst size.
// Starts a background goroutine to clean up idle buckets.
func NewLimiter(rpm, burst int) *Limiter {
	l := &Limiter{rpm: rpm, burst: burst}
	go l.cleanupLoop()
	return l
}

// Allow returns true if the given agentID is within rate limits.
func (l *Limiter) Allow(agentID string) bool {
	v, _ := l.buckets.LoadOrStore(agentID, newBucket(l.rpm, l.burst))
	return v.(*bucket).allow()
}

// Wrap returns an HTTP middleware that enforces rate limits per agent_id from context.
func (l *Limiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agentID, _ := r.Context().Value(gatewayctx.AgentID).(string)
		if agentID == "" {
			// Should not happen in practice (auth middleware runs first)
			agentID = "_unknown_"
		}
		if !l.Allow(agentID) {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// cleanupLoop periodically removes buckets that have been idle for > 5 minutes.
func (l *Limiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-5 * time.Minute)
		l.buckets.Range(func(k, v interface{}) bool {
			if v.(*bucket).lastActive().Before(cutoff) {
				l.buckets.Delete(k)
			}
			return true
		})
	}
}
