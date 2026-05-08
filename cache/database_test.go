package cache

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestDatabaseCacheSetGetDeleteAndExpire(t *testing.T) {
	ctx := context.Background()
	db := openTestSQLite(t)
	store, err := Open(ctx, StoreConfig{
		Driver: DriverDatabase,
		Database: DatabaseConfig{
			Driver:      "sqlite",
			Table:       "cache_items",
			AutoMigrate: true,
		},
	}, WithDatabase(db, "sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if err := store.Set(ctx, "key", []byte("value"), time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	got, err := store.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(got) != "value" {
		t.Fatalf("Get() = %q, want value", got)
	}
	exists, err := store.Exists(ctx, "key")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Fatalf("Exists() = false, want true")
	}

	if err := store.Delete(ctx, "key"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	exists, err = store.Exists(ctx, "key")
	if err != nil {
		t.Fatalf("Exists(deleted) error = %v", err)
	}
	if exists {
		t.Fatalf("Exists(deleted) = true, want false")
	}
	if _, err := store.Get(ctx, "key"); !errors.Is(err, ErrMiss) {
		t.Fatalf("deleted key error = %v, want ErrMiss", err)
	}

	if err := store.Set(ctx, "expired", []byte("value"), time.Nanosecond); err != nil {
		t.Fatalf("Set(expired) error = %v", err)
	}
	time.Sleep(time.Millisecond)
	if _, err := store.Get(ctx, "expired"); !errors.Is(err, ErrMiss) {
		t.Fatalf("expired key error = %v, want ErrMiss", err)
	}
}

func TestDatabaseCacheTake(t *testing.T) {
	ctx := context.Background()
	db := openTestSQLite(t)
	store, err := Open(ctx, StoreConfig{
		Driver: DriverDatabase,
		Database: DatabaseConfig{
			Driver:      "sqlite",
			Table:       "cache_items",
			AutoMigrate: true,
		},
	}, WithDatabase(db, "sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if err := store.Set(ctx, "key", []byte("value"), time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	got, err := store.Take(ctx, "key")
	if err != nil {
		t.Fatalf("Take() error = %v", err)
	}
	if string(got) != "value" {
		t.Fatalf("Take() = %q, want value", got)
	}
	if _, err := store.Get(ctx, "key"); !errors.Is(err, ErrMiss) {
		t.Fatalf("taken key error = %v, want ErrMiss", err)
	}
}

func TestDatabaseCacheIncrementAndTTL(t *testing.T) {
	ctx := context.Background()
	db := openTestSQLite(t)
	store, err := Open(ctx, StoreConfig{
		Driver: DriverDatabase,
		Database: DatabaseConfig{
			Driver:      "sqlite",
			Table:       "cache_items",
			AutoMigrate: true,
		},
	}, WithDatabase(db, "sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	got, err := store.Increment(ctx, "counter", 1, time.Minute)
	if err != nil {
		t.Fatalf("Increment() error = %v", err)
	}
	if got != 1 {
		t.Fatalf("Increment() = %d, want 1", got)
	}
	got, err = store.Increment(ctx, "counter", 4, time.Minute)
	if err != nil {
		t.Fatalf("Increment() again error = %v", err)
	}
	if got != 5 {
		t.Fatalf("Increment() = %d, want 5", got)
	}
	value, err := store.Get(ctx, "counter")
	if err != nil {
		t.Fatalf("Get(counter) error = %v", err)
	}
	if string(value) != "5" {
		t.Fatalf("counter value = %q, want 5", value)
	}
	ttl, err := store.TTL(ctx, "counter")
	if err != nil {
		t.Fatalf("TTL(counter) error = %v", err)
	}
	if ttl <= 0 || ttl > time.Minute {
		t.Fatalf("TTL(counter) = %v, want within (0, 1m]", ttl)
	}
}

func TestDatabaseCacheUsesNamedConnection(t *testing.T) {
	ctx := context.Background()
	db := openTestSQLite(t)

	store, err := Open(ctx, StoreConfig{
		Driver: DriverDatabase,
		Database: DatabaseConfig{
			Connection:  "cache",
			Table:       "cache_items",
			AutoMigrate: true,
		},
	}, WithDatabaseResolver(func(name string) (DatabaseHandle, error) {
		if name != "cache" {
			t.Fatalf("resolver name = %q, want cache", name)
		}
		return DatabaseHandle{DB: db, Driver: "sqlite"}, nil
	}))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if err := store.Set(ctx, "key", []byte("value"), 0); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
}

func TestDatabaseCacheManager(t *testing.T) {
	ctx := context.Background()
	db := openTestSQLite(t)
	manager, err := OpenManager(ctx, Config{
		DefaultStore: "db",
		Stores: map[string]StoreConfig{
			"db": {
				Driver: DriverDatabase,
				Database: DatabaseConfig{
					Connection:  "default",
					Table:       "cache_items",
					AutoMigrate: true,
				},
			},
		},
	}, WithDatabaseResolver(func(name string) (DatabaseHandle, error) {
		return DatabaseHandle{DB: db, Driver: "sqlite"}, nil
	}))
	if err != nil {
		t.Fatalf("OpenManager() error = %v", err)
	}
	defer manager.Close()

	if err := manager.Default().Set(ctx, "key", []byte("value"), 0); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	got, err := manager.Cache("db").Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(got) != "value" {
		t.Fatalf("Get() = %q, want value", got)
	}
}

func TestDatabaseCacheRequiresResolver(t *testing.T) {
	_, err := Open(context.Background(), StoreConfig{
		Driver: DriverDatabase,
		Database: DatabaseConfig{
			Driver: "sqlite",
		},
	})
	if !errors.Is(err, ErrDatabaseUnset) {
		t.Fatalf("Open() error = %v, want ErrDatabaseUnset", err)
	}
}

func TestDatabaseCacheRejectsUnsafeTableName(t *testing.T) {
	db := openTestSQLite(t)
	_, err := Open(context.Background(), StoreConfig{
		Driver: DriverDatabase,
		Database: DatabaseConfig{
			Driver:      "sqlite",
			Table:       "cache_items;drop",
			AutoMigrate: true,
		},
	}, WithDatabase(db, "sqlite"))
	if err == nil {
		t.Fatalf("Open() error = nil, want invalid identifier error")
	}
}

func openTestSQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}
