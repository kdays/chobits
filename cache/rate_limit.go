package cache

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

type RateLimitResult struct {
	Key   string
	Count int64
	Limit int64
	TTL   time.Duration
}

func (result RateLimitResult) Exceeded() bool {
	return result.Limit > 0 && result.Count >= result.Limit
}

type RateLimiter struct {
	store  Cache
	prefix string
}

func NewRateLimiter(store Cache, prefix string) *RateLimiter {
	if store == nil {
		store = NewMemory()
	}
	return &RateLimiter{
		store:  store,
		prefix: prefix,
	}
}

func (limiter *RateLimiter) Hit(ctx context.Context, key string, limit int64, window time.Duration) (RateLimitResult, error) {
	fullKey := limiter.key(key)
	count, err := limiter.store.Increment(ctx, fullKey, 1, window)
	if err != nil {
		return RateLimitResult{}, err
	}
	ttl, err := limiter.store.TTL(ctx, fullKey)
	if errors.Is(err, ErrMiss) {
		err = nil
	}
	if err != nil {
		return RateLimitResult{}, err
	}
	return RateLimitResult{
		Key:   fullKey,
		Count: count,
		Limit: limit,
		TTL:   ttl,
	}, nil
}

func (limiter *RateLimiter) Peek(ctx context.Context, key string, limit int64) (RateLimitResult, error) {
	fullKey := limiter.key(key)
	value, err := limiter.store.Get(ctx, fullKey)
	if errors.Is(err, ErrMiss) {
		return RateLimitResult{Key: fullKey, Limit: limit}, nil
	}
	if err != nil {
		return RateLimitResult{}, err
	}
	count, err := strconv.ParseInt(string(value), 10, 64)
	if err != nil {
		return RateLimitResult{}, err
	}
	ttl, err := limiter.store.TTL(ctx, fullKey)
	if errors.Is(err, ErrMiss) {
		ttl = 0
		err = nil
	}
	if err != nil {
		return RateLimitResult{}, err
	}
	return RateLimitResult{
		Key:   fullKey,
		Count: count,
		Limit: limit,
		TTL:   ttl,
	}, nil
}

func (limiter *RateLimiter) Reset(ctx context.Context, key string) error {
	return limiter.store.Delete(ctx, limiter.key(key))
}

func (limiter *RateLimiter) key(key string) string {
	key = strings.TrimSpace(key)
	return Key(limiter.prefix, key)
}
