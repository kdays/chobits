package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	DriverMemory   = "memory"
	DriverRedis    = "redis"
	DriverDatabase = "database"
)

type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Take(ctx context.Context, key string) ([]byte, error)
	Exists(ctx context.Context, key string) (bool, error)
	Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
	Delete(ctx context.Context, key string) error
	Close() error
}

type Config struct {
	DefaultStore string                 `yaml:"default_store"`
	Stores       map[string]StoreConfig `yaml:"stores"`
}

type StoreConfig struct {
	Driver   string         `yaml:"driver"`
	Prefix   string         `yaml:"prefix"`
	Required bool           `yaml:"required"`
	Memory   MemoryConfig   `yaml:"memory"`
	Redis    RedisConfig    `yaml:"redis"`
	Database DatabaseConfig `yaml:"database"`
}

type MemoryConfig struct{}

type DatabaseConfig struct {
	Connection  string `yaml:"connection"`
	Driver      string `yaml:"driver"`
	Table       string `yaml:"table"`
	AutoMigrate bool   `yaml:"auto_migrate"`
}

type DatabaseHandle struct {
	DB     *sql.DB
	Driver string
}

type DatabaseResolver func(name string) (DatabaseHandle, error)

type OpenOptions struct {
	databaseResolver DatabaseResolver
}

type OpenOption func(*OpenOptions)

type Manager struct {
	defaultStore string
	stores       map[string]Cache
}

func WithDatabaseResolver(resolver DatabaseResolver) OpenOption {
	return func(options *OpenOptions) {
		options.databaseResolver = resolver
	}
}

func WithDatabase(db *sql.DB, driver string) OpenOption {
	return WithDatabaseResolver(func(string) (DatabaseHandle, error) {
		return DatabaseHandle{DB: db, Driver: driver}, nil
	})
}

func (cfg *Config) ApplyDefaults() {
	if cfg.DefaultStore == "" {
		cfg.DefaultStore = "default"
	}
	if cfg.Stores == nil {
		cfg.Stores = map[string]StoreConfig{}
	}
	if len(cfg.Stores) == 0 {
		cfg.Stores[cfg.DefaultStore] = StoreConfig{Driver: DriverMemory}
	}
	for name, store := range cfg.Stores {
		store.ApplyDefaults()
		cfg.Stores[name] = store
	}
}

func (cfg *StoreConfig) ApplyDefaults() {
	if cfg.Driver == "" {
		cfg.Driver = DriverMemory
	}
	if strings.EqualFold(cfg.Driver, DriverRedis) && cfg.Redis.Addr == "" {
		cfg.Redis.Addr = "127.0.0.1:6379"
	}
	if strings.EqualFold(cfg.Driver, DriverDatabase) {
		cfg.Database.ApplyDefaults()
	}
}

func (cfg StoreConfig) IsRequired() bool {
	return cfg.Required || cfg.Redis.Required
}

func (cfg *DatabaseConfig) ApplyDefaults() {
	if cfg.Table == "" {
		cfg.Table = "chobits_cache"
	}
}

func (cfg Config) Store(name string) (StoreConfig, error) {
	cfg.ApplyDefaults()
	if strings.TrimSpace(name) == "" {
		name = cfg.DefaultStore
	}
	store, ok := cfg.Stores[name]
	if !ok {
		return StoreConfig{}, fmt.Errorf("%w: %s", ErrStoreNotFound, name)
	}
	store.ApplyDefaults()
	return store, nil
}

func OpenDefault(ctx context.Context, cfg Config, opts ...OpenOption) (Cache, error) {
	store, err := cfg.Store("")
	if err != nil {
		return nil, err
	}
	return Open(ctx, store, opts...)
}

func Open(ctx context.Context, cfg StoreConfig, opts ...OpenOption) (Cache, error) {
	options := openOptions(opts...)
	cfg.ApplyDefaults()
	switch strings.ToLower(strings.TrimSpace(cfg.Driver)) {
	case "", DriverMemory:
		return withStorePrefix(NewMemory(), cfg.Prefix), nil
	case DriverRedis:
		redisCfg := cfg.Redis
		if redisCfg.Prefix == "" {
			redisCfg.Prefix = cfg.Prefix
		}
		return OpenRedis(ctx, redisCfg)
	case DriverDatabase:
		databaseCfg := cfg.Database
		store, err := OpenDatabase(ctx, databaseCfg, options.databaseResolver)
		if err != nil {
			return nil, err
		}
		return withStorePrefix(store, cfg.Prefix), nil
	default:
		return nil, fmt.Errorf("unsupported cache driver %q", cfg.Driver)
	}
}

func OpenManager(ctx context.Context, cfg Config, opts ...OpenOption) (*Manager, error) {
	options := openOptions(opts...)
	cfg.ApplyDefaults()
	stores := make(map[string]Cache, len(cfg.Stores))
	for name, storeCfg := range cfg.Stores {
		store, err := Open(ctx, storeCfg, WithDatabaseResolver(options.databaseResolver))
		if err != nil {
			_ = closeAll(stores)
			return nil, fmt.Errorf("open cache store %s: %w", name, err)
		}
		stores[name] = store
	}
	if len(stores) == 0 {
		stores["default"] = NewMemory()
		cfg.DefaultStore = "default"
	}
	if _, ok := stores[cfg.DefaultStore]; !ok {
		_ = closeAll(stores)
		return nil, fmt.Errorf("%w: %s", ErrStoreNotFound, cfg.DefaultStore)
	}
	return &Manager{
		defaultStore: cfg.DefaultStore,
		stores:       stores,
	}, nil
}

func (manager *Manager) Default() Cache {
	if manager == nil {
		return nil
	}
	return manager.stores[manager.defaultStore]
}

func (manager *Manager) Cache(name string) Cache {
	if manager == nil {
		return nil
	}
	if strings.TrimSpace(name) == "" {
		name = manager.defaultStore
	}
	return manager.stores[name]
}

func (manager *Manager) Names() []string {
	if manager == nil {
		return nil
	}
	names := make([]string, 0, len(manager.stores))
	for name := range manager.stores {
		names = append(names, name)
	}
	return names
}

func (manager *Manager) Close() error {
	if manager == nil {
		return nil
	}
	return closeAll(manager.stores)
}

func closeAll(stores map[string]Cache) error {
	var result error
	for _, store := range stores {
		if store == nil {
			continue
		}
		result = errors.Join(result, store.Close())
	}
	return result
}

func openOptions(opts ...OpenOption) OpenOptions {
	var options OpenOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	return options
}
