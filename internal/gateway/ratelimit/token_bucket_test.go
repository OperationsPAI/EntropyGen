package ratelimit

import (
	"testing"
	"time"
)

func TestLimiter_BurstAllowed(t *testing.T) {
	l := NewLimiter(60, 10)
	agentID := "agent-burst-test"

	// All burst tokens should be available immediately
	for i := 0; i < 10; i++ {
		if !l.Allow(agentID) {
			t.Errorf("request %d should be allowed (within burst=%d)", i+1, 10)
		}
	}
}

func TestLimiter_BlockedAfterBurst(t *testing.T) {
	l := NewLimiter(60, 10)
	agentID := "agent-block-test"

	for i := 0; i < 10; i++ {
		l.Allow(agentID)
	}
	// 11th request should be blocked
	if l.Allow(agentID) {
		t.Error("11th request should be rate-limited after burst exhausted")
	}
}

func TestLimiter_IndependentAgents(t *testing.T) {
	l := NewLimiter(60, 10)

	// Exhaust agent-1
	for i := 0; i < 11; i++ {
		l.Allow("agent-1")
	}
	// agent-2 should be unaffected
	if !l.Allow("agent-2") {
		t.Error("agent-2 should not be rate-limited by agent-1's quota")
	}
}

func TestLimiter_RefillOverTime(t *testing.T) {
	// This test validates the bucket's refill math, not wall-clock time.
	// Create a bucket directly and simulate time passage.
	b := newBucket(60, 5) // 60 rpm = 1/sec, burst=5

	// Drain all tokens
	for i := 0; i < 5; i++ {
		b.allow()
	}
	if b.allow() {
		t.Error("bucket should be empty")
	}

	// Simulate 2 seconds passing
	b.mu.Lock()
	b.lastRefill = b.lastRefill.Add(-2 * time.Second)
	b.mu.Unlock()

	// Should have ~2 tokens refilled (1/sec rate)
	if !b.allow() {
		t.Error("should have refilled token after 2 seconds")
	}
}
