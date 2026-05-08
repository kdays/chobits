package redis

import (
	"context"

	"github.com/kdays/chobits/cache"
	"github.com/kdays/chobits/config"
	goredis "github.com/redis/go-redis/v9"
)

func Open(ctx context.Context, cfg config.Redis) (*goredis.Client, error) {
	return cache.OpenRedisClient(ctx, cache.RedisConfig{
		Addr:     cfg.Addr,
		Username: cfg.Username,
		Password: cfg.Password,
		DB:       cfg.DB,
		Prefix:   cfg.Prefix,
		Required: cfg.Required,
	})
}

func Key(prefix, key string) string {
	return cache.Key(prefix, key)
}
