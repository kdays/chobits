package cache

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestMemoryCopiesValuesAndExpires(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	store := NewMemory()
	store.now = func() time.Time { return now }

	value := []byte("hello")
	if err := store.Set(ctx, "key", value, time.Minute); err != nil {
		t.Fatalf("set value: %v", err)
	}
	value[0] = 'x'

	got, err := store.Get(ctx, "key")
	if err != nil {
		t.Fatalf("get value: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("stored value was mutated through input slice: %q", got)
	}

	got[1] = 'x'
	got, err = store.Get(ctx, "key")
	if err != nil {
		t.Fatalf("get value again: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("stored value was mutated through returned slice: %q", got)
	}

	now = now.Add(time.Minute)
	exists, err := store.Exists(ctx, "key")
	if err != nil {
		t.Fatalf("exists expired value: %v", err)
	}
	if exists {
		t.Fatalf("expired value should not exist")
	}
	_, err = store.Get(ctx, "key")
	if !errors.Is(err, ErrMiss) {
		t.Fatalf("expected expired value to miss, got %v", err)
	}
}

func TestMemoryDelete(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	if err := store.Set(ctx, "key", []byte("value"), 0); err != nil {
		t.Fatalf("set value: %v", err)
	}
	exists, err := store.Exists(ctx, "key")
	if err != nil {
		t.Fatalf("exists value: %v", err)
	}
	if !exists {
		t.Fatalf("expected value to exist")
	}
	if err := store.Delete(ctx, "key"); err != nil {
		t.Fatalf("delete value: %v", err)
	}
	exists, err = store.Exists(ctx, "key")
	if err != nil {
		t.Fatalf("exists deleted value: %v", err)
	}
	if exists {
		t.Fatalf("deleted value should not exist")
	}
	_, err = store.Get(ctx, "key")
	if !errors.Is(err, ErrMiss) {
		t.Fatalf("expected deleted value to miss, got %v", err)
	}
}

func TestMemoryTake(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	if err := store.Set(ctx, "key", []byte("value"), 0); err != nil {
		t.Fatalf("set value: %v", err)
	}
	got, err := store.Take(ctx, "key")
	if err != nil {
		t.Fatalf("take value: %v", err)
	}
	if string(got) != "value" {
		t.Fatalf("Take() = %q, want value", got)
	}
	if _, err := store.Get(ctx, "key"); !errors.Is(err, ErrMiss) {
		t.Fatalf("taken key error = %v, want ErrMiss", err)
	}
	if _, err := store.Take(ctx, "key"); !errors.Is(err, ErrMiss) {
		t.Fatalf("retake key error = %v, want ErrMiss", err)
	}
}

func TestMemoryIncrementAndTTL(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	store := NewMemory()
	store.now = func() time.Time { return now }

	got, err := store.Increment(ctx, "counter", 1, time.Minute)
	if err != nil {
		t.Fatalf("increment counter: %v", err)
	}
	if got != 1 {
		t.Fatalf("increment = %d, want 1", got)
	}
	got, err = store.Increment(ctx, "counter", 4, time.Minute)
	if err != nil {
		t.Fatalf("increment counter again: %v", err)
	}
	if got != 5 {
		t.Fatalf("increment = %d, want 5", got)
	}
	value, err := store.Get(ctx, "counter")
	if err != nil {
		t.Fatalf("get counter: %v", err)
	}
	if string(value) != "5" {
		t.Fatalf("counter value = %q, want 5", value)
	}
	ttl, err := store.TTL(ctx, "counter")
	if err != nil {
		t.Fatalf("ttl counter: %v", err)
	}
	if ttl != time.Minute {
		t.Fatalf("ttl = %v, want %v", ttl, time.Minute)
	}

	now = now.Add(time.Minute)
	if _, err := store.TTL(ctx, "counter"); !errors.Is(err, ErrMiss) {
		t.Fatalf("expired ttl error = %v, want ErrMiss", err)
	}
}

func TestFallbackReadsFallbackAndWritesBoth(t *testing.T) {
	ctx := context.Background()
	primary := NewMemory()
	secondary := NewMemory()
	store := NewFallback(primary, secondary)

	if err := secondary.Set(ctx, "key", []byte("secondary"), 0); err != nil {
		t.Fatalf("seed fallback: %v", err)
	}
	got, err := store.Get(ctx, "key")
	if err != nil {
		t.Fatalf("get fallback value: %v", err)
	}
	if string(got) != "secondary" {
		t.Fatalf("expected fallback value, got %q", got)
	}

	if err := store.Set(ctx, "key", []byte("updated"), time.Minute); err != nil {
		t.Fatalf("set fallback cache: %v", err)
	}
	for name, cache := range map[string]*Memory{"primary": primary, "fallback": secondary} {
		got, err := cache.Get(ctx, "key")
		if err != nil {
			t.Fatalf("get %s value: %v", name, err)
		}
		if string(got) != "updated" {
			t.Fatalf("expected %s to be updated, got %q", name, got)
		}
	}
}

func TestFallbackIncrementMirrorsValue(t *testing.T) {
	ctx := context.Background()
	primary := NewMemory()
	secondary := NewMemory()
	store := NewFallback(primary, secondary)

	got, err := store.Increment(ctx, "counter", 2, time.Minute)
	if err != nil {
		t.Fatalf("increment fallback cache: %v", err)
	}
	if got != 2 {
		t.Fatalf("increment = %d, want 2", got)
	}
	for name, cache := range map[string]*Memory{"primary": primary, "fallback": secondary} {
		value, err := cache.Get(ctx, "counter")
		if err != nil {
			t.Fatalf("get %s counter: %v", name, err)
		}
		if string(value) != "2" {
			t.Fatalf("%s counter = %q, want 2", name, value)
		}
	}
}

func TestFallbackUsesFallbackWhenPrimaryErrors(t *testing.T) {
	ctx := context.Background()
	secondary := NewMemory()
	if err := secondary.Set(ctx, "key", []byte("secondary"), 0); err != nil {
		t.Fatalf("seed fallback: %v", err)
	}

	store := NewFallback(errorCache{err: errors.New("primary down")}, secondary)
	got, err := store.Get(ctx, "key")
	if err != nil {
		t.Fatalf("get fallback value after primary error: %v", err)
	}
	if string(got) != "secondary" {
		t.Fatalf("expected fallback value, got %q", got)
	}
}

func TestConfigDefaultsToMemoryStore(t *testing.T) {
	var cfg Config
	cfg.ApplyDefaults()

	if cfg.DefaultStore != "default" {
		t.Fatalf("default store = %q, want default", cfg.DefaultStore)
	}
	store, err := cfg.Store("")
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	if store.Driver != DriverMemory {
		t.Fatalf("driver = %q, want memory", store.Driver)
	}
}

func TestManagerOpensNamedMemoryStores(t *testing.T) {
	ctx := context.Background()
	manager, err := OpenManager(ctx, Config{
		DefaultStore: "default",
		Stores: map[string]StoreConfig{
			"default":  {Driver: DriverMemory, Prefix: "app"},
			"sessions": {Driver: DriverMemory},
		},
	})
	if err != nil {
		t.Fatalf("OpenManager() error = %v", err)
	}
	defer manager.Close()

	names := manager.Names()
	sort.Strings(names)
	if got := strings.Join(names, ","); got != "default,sessions" {
		t.Fatalf("names = %q, want default,sessions", got)
	}

	if err := manager.Default().Set(ctx, "key", []byte("value"), 0); err != nil {
		t.Fatalf("set default cache: %v", err)
	}
	if _, err := manager.Cache("sessions").Get(ctx, "key"); !errors.Is(err, ErrMiss) {
		t.Fatalf("session cache should be independent, got %v", err)
	}
	if got, err := manager.Default().Get(ctx, "key"); err != nil || string(got) != "value" {
		t.Fatalf("default cache get = %q, %v; want value, nil", got, err)
	}
	if exists, err := manager.Default().Exists(ctx, "key"); err != nil || !exists {
		t.Fatalf("default cache exists = %v, %v; want true, nil", exists, err)
	}
}

func TestManagerRejectsMissingDefaultStore(t *testing.T) {
	ctx := context.Background()
	_, err := OpenManager(ctx, Config{
		DefaultStore: "missing",
		Stores: map[string]StoreConfig{
			"default": {Driver: DriverMemory},
		},
	})
	if !errors.Is(err, ErrStoreNotFound) {
		t.Fatalf("OpenManager() error = %v, want ErrStoreNotFound", err)
	}
}

func TestPrefixedCache(t *testing.T) {
	ctx := context.Background()
	memory := NewMemory()
	store := NewPrefixed(memory, "app")

	if err := store.Set(ctx, "key", []byte("value"), 0); err != nil {
		t.Fatalf("set prefixed cache: %v", err)
	}
	if _, err := memory.Get(ctx, "key"); !errors.Is(err, ErrMiss) {
		t.Fatalf("unprefixed key should miss, got %v", err)
	}
	if got, err := memory.Get(ctx, "app:key"); err != nil || string(got) != "value" {
		t.Fatalf("prefixed key get = %q, %v; want value, nil", got, err)
	}
}

func TestKey(t *testing.T) {
	tests := []struct {
		prefix string
		key    string
		want   string
	}{
		{prefix: "", key: "user:1", want: "user:1"},
		{prefix: "app", key: "user:1", want: "app:user:1"},
		{prefix: "app:", key: "user:1", want: "app:user:1"},
	}

	for _, tt := range tests {
		if got := Key(tt.prefix, tt.key); got != tt.want {
			t.Fatalf("Key(%q, %q) = %q, want %q", tt.prefix, tt.key, got, tt.want)
		}
	}
}

type errorCache struct {
	err error
}

func (cache errorCache) Get(context.Context, string) ([]byte, error) {
	return nil, cache.err
}

func (cache errorCache) Set(context.Context, string, []byte, time.Duration) error {
	return cache.err
}

func (cache errorCache) Take(context.Context, string) ([]byte, error) {
	return nil, cache.err
}

func (cache errorCache) Exists(context.Context, string) (bool, error) {
	return false, cache.err
}

func (cache errorCache) Increment(context.Context, string, int64, time.Duration) (int64, error) {
	return 0, cache.err
}

func (cache errorCache) TTL(context.Context, string) (time.Duration, error) {
	return 0, cache.err
}

func (cache errorCache) Delete(context.Context, string) error {
	return cache.err
}

func (cache errorCache) Close() error {
	return cache.err
}
