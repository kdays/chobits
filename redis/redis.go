package redis

import (
	"context"

	"github.com/kdays/chobits/cache"
	"github.com/kdays/chobits/config"
)

func Open(ctx context.Context, cfg config.Redis) (cache.RedisClient, error) {
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
