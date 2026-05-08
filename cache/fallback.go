package cache

import (
	"context"
	"errors"
	"strconv"
	"time"
)

type Fallback struct {
	primary  Cache
	fallback Cache
}

func NewFallback(primary Cache, fallback Cache) *Fallback {
	return &Fallback{
		primary:  primary,
		fallback: fallback,
	}
}

func (cache *Fallback) Get(ctx context.Context, key string) ([]byte, error) {
	if cache.primary != nil {
		value, err := cache.primary.Get(ctx, key)
		if err == nil {
			return value, err
		}
		if !errors.Is(err, ErrMiss) && cache.fallback == nil {
			return nil, err
		}
	}
	if cache.fallback == nil {
		return nil, ErrMiss
	}
	return cache.fallback.Get(ctx, key)
}

func (cache *Fallback) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	var result error
	if cache.primary != nil {
		result = errors.Join(result, cache.primary.Set(ctx, key, value, ttl))
	}
	if cache.fallback != nil {
		result = errors.Join(result, cache.fallback.Set(ctx, key, value, ttl))
	}
	return result
}

func (cache *Fallback) Take(ctx context.Context, key string) ([]byte, error) {
	if cache.primary != nil {
		value, err := cache.primary.Take(ctx, key)
		if err == nil {
			if cache.fallback != nil {
				_ = cache.fallback.Delete(ctx, key)
			}
			return value, nil
		}
		if !errors.Is(err, ErrMiss) && cache.fallback == nil {
			return nil, err
		}
	}
	if cache.fallback == nil {
		return nil, ErrMiss
	}
	return cache.fallback.Take(ctx, key)
}

func (cache *Fallback) Exists(ctx context.Context, key string) (bool, error) {
	if cache.primary != nil {
		ok, err := cache.primary.Exists(ctx, key)
		if ok {
			return true, nil
		}
		if err != nil && cache.fallback == nil {
			return false, err
		}
		if err == nil && cache.fallback == nil {
			return false, nil
		}
	}
	if cache.fallback == nil {
		return false, nil
	}
	return cache.fallback.Exists(ctx, key)
}

func (cache *Fallback) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	if cache.primary != nil {
		next, err := cache.primary.Increment(ctx, key, delta, ttl)
		if err == nil {
			if cache.fallback != nil {
				_ = cache.fallback.Set(ctx, key, []byte(strconv.FormatInt(next, 10)), ttl)
			}
			return next, nil
		}
		if cache.fallback == nil {
			return 0, err
		}
	}
	if cache.fallback == nil {
		return 0, ErrMiss
	}
	return cache.fallback.Increment(ctx, key, delta, ttl)
}

func (cache *Fallback) TTL(ctx context.Context, key string) (time.Duration, error) {
	if cache.primary != nil {
		ttl, err := cache.primary.TTL(ctx, key)
		if err == nil {
			return ttl, nil
		}
		if !errors.Is(err, ErrMiss) && cache.fallback == nil {
			return 0, err
		}
	}
	if cache.fallback == nil {
		return 0, ErrMiss
	}
	return cache.fallback.TTL(ctx, key)
}

func (cache *Fallback) Delete(ctx context.Context, key string) error {
	var result error
	if cache.primary != nil {
		result = errors.Join(result, cache.primary.Delete(ctx, key))
	}
	if cache.fallback != nil {
		result = errors.Join(result, cache.fallback.Delete(ctx, key))
	}
	return result
}

func (cache *Fallback) Close() error {
	var result error
	if cache.primary != nil {
		result = errors.Join(result, cache.primary.Close())
	}
	if cache.fallback != nil {
		result = errors.Join(result, cache.fallback.Close())
	}
	return result
}
