package cache

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"
)

type Memory struct {
	mu    sync.RWMutex
	items map[string]memoryItem
	now   func() time.Time
}

type memoryItem struct {
	value     []byte
	expiresAt time.Time
}

func NewMemory() *Memory {
	return &Memory{
		items: make(map[string]memoryItem),
		now:   time.Now,
	}
}

func (cache *Memory) Get(_ context.Context, key string) ([]byte, error) {
	cache.mu.RLock()
	item, ok := cache.items[key]
	cache.mu.RUnlock()
	if !ok {
		return nil, ErrMiss
	}
	if !item.expiresAt.IsZero() && !item.expiresAt.After(cache.now()) {
		cache.mu.Lock()
		delete(cache.items, key)
		cache.mu.Unlock()
		return nil, ErrMiss
	}
	value := make([]byte, len(item.value))
	copy(value, item.value)
	return value, nil
}

func (cache *Memory) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	copied := make([]byte, len(value))
	copy(copied, value)

	item := memoryItem{value: copied}
	if ttl > 0 {
		item.expiresAt = cache.now().Add(ttl)
	}

	cache.mu.Lock()
	cache.items[key] = item
	cache.mu.Unlock()
	return nil
}

func (cache *Memory) Take(_ context.Context, key string) ([]byte, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	item, ok := cache.items[key]
	if !ok {
		return nil, ErrMiss
	}
	if !item.expiresAt.IsZero() && !item.expiresAt.After(cache.now()) {
		delete(cache.items, key)
		return nil, ErrMiss
	}
	delete(cache.items, key)
	value := make([]byte, len(item.value))
	copy(value, item.value)
	return value, nil
}

func (cache *Memory) Exists(ctx context.Context, key string) (bool, error) {
	if _, err := cache.Get(ctx, key); err != nil {
		if errors.Is(err, ErrMiss) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (cache *Memory) Increment(_ context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	item, ok := cache.items[key]
	now := cache.now()
	if ok && !item.expiresAt.IsZero() && !item.expiresAt.After(now) {
		delete(cache.items, key)
		ok = false
		item = memoryItem{}
	}

	current := int64(0)
	if ok {
		parsed, err := strconv.ParseInt(string(item.value), 10, 64)
		if err != nil {
			return 0, err
		}
		current = parsed
	}

	next := current + delta
	expiresAt := item.expiresAt
	if ttl > 0 {
		expiresAt = now.Add(ttl)
	}
	cache.items[key] = memoryItem{
		value:     []byte(strconv.FormatInt(next, 10)),
		expiresAt: expiresAt,
	}
	return next, nil
}

func (cache *Memory) TTL(_ context.Context, key string) (time.Duration, error) {
	cache.mu.RLock()
	item, ok := cache.items[key]
	cache.mu.RUnlock()
	if !ok {
		return 0, ErrMiss
	}

	if item.expiresAt.IsZero() {
		return 0, nil
	}
	ttl := item.expiresAt.Sub(cache.now())
	if ttl <= 0 {
		cache.mu.Lock()
		delete(cache.items, key)
		cache.mu.Unlock()
		return 0, ErrMiss
	}
	return ttl, nil
}

func (cache *Memory) Delete(_ context.Context, key string) error {
	cache.mu.Lock()
	delete(cache.items, key)
	cache.mu.Unlock()
	return nil
}

func (cache *Memory) Close() error {
	return nil
}
