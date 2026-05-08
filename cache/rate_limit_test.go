package cache

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiterHitPeekAndReset(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	limiter := NewRateLimiter(store, "retry")

	result, err := limiter.Hit(ctx, "admin:login", 3, time.Minute)
	if err != nil {
		t.Fatalf("Hit() error = %v", err)
	}
	if result.Key != "retry:admin:login" {
		t.Fatalf("key = %q, want retry:admin:login", result.Key)
	}
	if result.Count != 1 {
		t.Fatalf("count = %d, want 1", result.Count)
	}
	if result.Exceeded() {
		t.Fatalf("first hit should not exceed limit")
	}

	if _, err := limiter.Hit(ctx, "admin:login", 3, time.Minute); err != nil {
		t.Fatalf("Hit() again error = %v", err)
	}
	result, err = limiter.Hit(ctx, "admin:login", 3, time.Minute)
	if err != nil {
		t.Fatalf("Hit() third time error = %v", err)
	}
	if result.Count != 3 {
		t.Fatalf("count = %d, want 3", result.Count)
	}
	if !result.Exceeded() {
		t.Fatalf("third hit should reach limit")
	}
	if result.TTL <= 0 || result.TTL > time.Minute {
		t.Fatalf("ttl = %v, want within (0, 1m]", result.TTL)
	}

	peek, err := limiter.Peek(ctx, "admin:login", 3)
	if err != nil {
		t.Fatalf("Peek() error = %v", err)
	}
	if peek.Count != 3 || !peek.Exceeded() {
		t.Fatalf("peek = %+v, want count 3 and exceeded", peek)
	}

	if err := limiter.Reset(ctx, "admin:login"); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}
	peek, err = limiter.Peek(ctx, "admin:login", 3)
	if err != nil {
		t.Fatalf("Peek() after reset error = %v", err)
	}
	if peek.Count != 0 || peek.Exceeded() {
		t.Fatalf("peek after reset = %+v, want zero count", peek)
	}
}
