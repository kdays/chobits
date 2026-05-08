package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

var incrementScript = goredis.NewScript(`
local value = redis.call("INCRBY", KEYS[1], ARGV[1])
local ttl = tonumber(ARGV[2])
if ttl and ttl > 0 then
	redis.call("PEXPIRE", KEYS[1], ttl)
end
return value
`)

var takeScript = goredis.NewScript(`
local value = redis.call("GET", KEYS[1])
if not value then
	return nil
end
redis.call("DEL", KEYS[1])
return value
`)

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	Prefix   string `yaml:"prefix"`
	Required bool   `yaml:"required"`
}

type Redis struct {
	client      goredis.UniversalClient
	prefix      string
	closeClient bool
}

func OpenRedis(ctx context.Context, cfg RedisConfig) (*Redis, error) {
	client, err := OpenRedisClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return NewOwnedRedis(client, cfg.Prefix), nil
}

func OpenRedisClient(ctx context.Context, cfg RedisConfig) (*goredis.Client, error) {
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:6379"
	}
	client := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr,
		Username: cfg.Username,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if ctx == nil {
		ctx = context.Background()
	}
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return client, nil
}

func NewRedis(client goredis.UniversalClient, prefix string) *Redis {
	return &Redis{
		client: client,
		prefix: prefix,
	}
}

func NewOwnedRedis(client goredis.UniversalClient, prefix string) *Redis {
	return &Redis{
		client:      client,
		prefix:      prefix,
		closeClient: true,
	}
}

func (cache *Redis) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := cache.client.Get(ctx, cache.key(key)).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, ErrMiss
		}
		return nil, err
	}
	return value, nil
}

func (cache *Redis) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return cache.client.Set(ctx, cache.key(key), value, ttl).Err()
}

func (cache *Redis) Take(ctx context.Context, key string) ([]byte, error) {
	result, err := takeScript.Run(ctx, cache.client, []string{cache.key(key)}).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, ErrMiss
		}
		return nil, err
	}
	switch value := result.(type) {
	case string:
		return []byte(value), nil
	case []byte:
		copied := make([]byte, len(value))
		copy(copied, value)
		return copied, nil
	default:
		return nil, ErrMiss
	}
}

func (cache *Redis) Exists(ctx context.Context, key string) (bool, error) {
	n, err := cache.client.Exists(ctx, cache.key(key)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (cache *Redis) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	ttlMillis := ttl.Milliseconds()
	if ttl > 0 && ttlMillis == 0 {
		ttlMillis = 1
	}
	result, err := incrementScript.Run(ctx, cache.client, []string{cache.key(key)}, delta, ttlMillis).Int64()
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (cache *Redis) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := cache.client.PTTL(ctx, cache.key(key)).Result()
	if err != nil {
		return 0, err
	}
	switch ttl {
	case -2 * time.Millisecond:
		return 0, ErrMiss
	case -1 * time.Millisecond:
		return 0, nil
	default:
		return ttl, nil
	}
}

func (cache *Redis) Delete(ctx context.Context, key string) error {
	return cache.client.Del(ctx, cache.key(key)).Err()
}

func (cache *Redis) Close() error {
	if !cache.closeClient {
		return nil
	}
	return cache.client.Close()
}

func (cache *Redis) Client() goredis.UniversalClient {
	if cache == nil {
		return nil
	}
	return cache.client
}

func (cache *Redis) key(key string) string {
	return Key(cache.prefix, key)
}

func Key(prefix, key string) string {
	prefix = strings.TrimRight(prefix, ":")
	if prefix == "" {
		return key
	}
	return prefix + ":" + key
}
