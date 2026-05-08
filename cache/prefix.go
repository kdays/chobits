package cache

import (
	"context"
	"strings"
	"time"
)

type Prefixed struct {
	store  Cache
	prefix string
}

func NewPrefixed(store Cache, prefix string) *Prefixed {
	return &Prefixed{
		store:  store,
		prefix: prefix,
	}
}

func (cache *Prefixed) Get(ctx context.Context, key string) ([]byte, error) {
	return cache.store.Get(ctx, cache.key(key))
}

func (cache *Prefixed) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return cache.store.Set(ctx, cache.key(key), value, ttl)
}

func (cache *Prefixed) Take(ctx context.Context, key string) ([]byte, error) {
	return cache.store.Take(ctx, cache.key(key))
}

func (cache *Prefixed) Exists(ctx context.Context, key string) (bool, error) {
	return cache.store.Exists(ctx, cache.key(key))
}

func (cache *Prefixed) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return cache.store.Increment(ctx, cache.key(key), delta, ttl)
}

func (cache *Prefixed) TTL(ctx context.Context, key string) (time.Duration, error) {
	return cache.store.TTL(ctx, cache.key(key))
}

func (cache *Prefixed) Delete(ctx context.Context, key string) error {
	return cache.store.Delete(ctx, cache.key(key))
}

func (cache *Prefixed) Close() error {
	return cache.store.Close()
}

func (cache *Prefixed) key(key string) string {
	return Key(cache.prefix, key)
}

func withStorePrefix(store Cache, prefix string) Cache {
	if strings.TrimSpace(prefix) == "" {
		return store
	}
	return NewPrefixed(store, prefix)
}
