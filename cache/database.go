package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Database struct {
	db     *sql.DB
	driver string
	table  string
	now    func() time.Time
}

func OpenDatabase(ctx context.Context, cfg DatabaseConfig, resolver DatabaseResolver) (*Database, error) {
	cfg.ApplyDefaults()
	if resolver == nil {
		return nil, ErrDatabaseUnset
	}
	handle, err := resolver(cfg.Connection)
	if err != nil {
		return nil, err
	}
	driver := cfg.Driver
	if driver == "" {
		driver = handle.Driver
	}
	store, err := NewDatabase(handle.DB, driver, cfg.Table)
	if err != nil {
		return nil, err
	}
	if cfg.AutoMigrate {
		if err := store.Migrate(ctx); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func NewDatabase(db *sql.DB, driver string, table string) (*Database, error) {
	if db == nil {
		return nil, ErrDatabaseUnset
	}
	table, err := cleanIdentifier(table)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(driver) == "" {
		driver = "mysql"
	}
	driver = normalizeDriver(driver)
	switch driver {
	case "mysql", "sqlite":
	default:
		return nil, fmt.Errorf("unsupported database cache driver %q", driver)
	}
	return &Database{
		db:     db,
		driver: driver,
		table:  table,
		now:    time.Now,
	}, nil
}

func (cache *Database) Migrate(ctx context.Context) error {
	if cache == nil || cache.db == nil {
		return ErrDatabaseUnset
	}
	query := ""
	switch cache.driver {
	case "sqlite", "sqlite3":
		query = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
	cache_key TEXT PRIMARY KEY,
	cache_value BLOB NOT NULL,
	expires_at INTEGER NOT NULL DEFAULT 0
)`, cache.table)
	case "", "mysql":
		query = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
	cache_key VARCHAR(512) NOT NULL PRIMARY KEY,
	cache_value LONGBLOB NOT NULL,
	expires_at BIGINT NOT NULL DEFAULT 0
)`, cache.table)
	default:
		return fmt.Errorf("unsupported database cache driver %q", cache.driver)
	}
	_, err := cache.db.ExecContext(ensureContext(ctx), query)
	if err != nil {
		return fmt.Errorf("migrate database cache: %w", err)
	}
	return nil
}

func (cache *Database) Get(ctx context.Context, key string) ([]byte, error) {
	if cache == nil || cache.db == nil {
		return nil, ErrDatabaseUnset
	}
	row := cache.db.QueryRowContext(ensureContext(ctx),
		fmt.Sprintf("SELECT cache_value, expires_at FROM %s WHERE cache_key = ?", cache.table),
		key,
	)
	var value []byte
	var expiresAt int64
	if err := row.Scan(&value, &expiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMiss
		}
		return nil, err
	}
	if expiresAt > 0 && expiresAt <= cache.now().UnixNano() {
		_ = cache.Delete(ctx, key)
		return nil, ErrMiss
	}
	copied := make([]byte, len(value))
	copy(copied, value)
	return copied, nil
}

func (cache *Database) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if cache == nil || cache.db == nil {
		return ErrDatabaseUnset
	}
	expiresAt := int64(0)
	if ttl > 0 {
		expiresAt = cache.now().Add(ttl).UnixNano()
	}

	ctx = ensureContext(ctx)
	tx, err := cache.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE cache_key = ?", cache.table), key); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf("INSERT INTO %s (cache_key, cache_value, expires_at) VALUES (?, ?, ?)", cache.table),
		key,
		value,
		expiresAt,
	); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (cache *Database) Take(ctx context.Context, key string) ([]byte, error) {
	if cache == nil || cache.db == nil {
		return nil, ErrDatabaseUnset
	}

	ctx = ensureContext(ctx)
	tx, err := cache.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf("SELECT cache_value, expires_at FROM %s WHERE cache_key = ?", cache.table)
	if cache.driver == "mysql" {
		query += " FOR UPDATE"
	}
	row := tx.QueryRowContext(ctx, query, key)
	var value []byte
	var expiresAt int64
	if err := row.Scan(&value, &expiresAt); err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMiss
		}
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE cache_key = ?", cache.table), key); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if expiresAt > 0 && expiresAt <= cache.now().UnixNano() {
		return nil, ErrMiss
	}
	copied := make([]byte, len(value))
	copy(copied, value)
	return copied, nil
}

func (cache *Database) Exists(ctx context.Context, key string) (bool, error) {
	if _, err := cache.Get(ctx, key); err != nil {
		if errors.Is(err, ErrMiss) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (cache *Database) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	if cache == nil || cache.db == nil {
		return 0, ErrDatabaseUnset
	}

	ctx = ensureContext(ctx)
	tx, err := cache.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}

	query := fmt.Sprintf("SELECT cache_value, expires_at FROM %s WHERE cache_key = ?", cache.table)
	if cache.driver == "mysql" {
		query += " FOR UPDATE"
	}
	row := tx.QueryRowContext(ctx, query, key)
	var raw []byte
	var expiresAt int64
	exists := true
	if err := row.Scan(&raw, &expiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			exists = false
		} else {
			_ = tx.Rollback()
			return 0, err
		}
	}

	now := cache.now()
	if exists && expiresAt > 0 && expiresAt <= now.UnixNano() {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE cache_key = ?", cache.table), key); err != nil {
			_ = tx.Rollback()
			return 0, err
		}
		exists = false
		expiresAt = 0
		raw = nil
	}

	current := int64(0)
	if exists {
		current, err = strconv.ParseInt(string(raw), 10, 64)
		if err != nil {
			_ = tx.Rollback()
			return 0, err
		}
	}

	next := current + delta
	if ttl > 0 {
		expiresAt = now.Add(ttl).UnixNano()
	}
	value := []byte(strconv.FormatInt(next, 10))
	if exists {
		_, err = tx.ExecContext(ctx,
			fmt.Sprintf("UPDATE %s SET cache_value = ?, expires_at = ? WHERE cache_key = ?", cache.table),
			value,
			expiresAt,
			key,
		)
	} else {
		_, err = tx.ExecContext(ctx,
			fmt.Sprintf("INSERT INTO %s (cache_key, cache_value, expires_at) VALUES (?, ?, ?)", cache.table),
			key,
			value,
			expiresAt,
		)
	}
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return next, nil
}

func (cache *Database) TTL(ctx context.Context, key string) (time.Duration, error) {
	if cache == nil || cache.db == nil {
		return 0, ErrDatabaseUnset
	}
	row := cache.db.QueryRowContext(ensureContext(ctx),
		fmt.Sprintf("SELECT expires_at FROM %s WHERE cache_key = ?", cache.table),
		key,
	)
	var expiresAt int64
	if err := row.Scan(&expiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrMiss
		}
		return 0, err
	}
	if expiresAt == 0 {
		return 0, nil
	}
	ttl := time.Duration(expiresAt - cache.now().UnixNano())
	if ttl <= 0 {
		_ = cache.Delete(ctx, key)
		return 0, ErrMiss
	}
	return ttl, nil
}

func (cache *Database) Delete(ctx context.Context, key string) error {
	if cache == nil || cache.db == nil {
		return ErrDatabaseUnset
	}
	_, err := cache.db.ExecContext(ensureContext(ctx), fmt.Sprintf("DELETE FROM %s WHERE cache_key = ?", cache.table), key)
	return err
}

func (cache *Database) Close() error {
	return nil
}

func normalizeDriver(driver string) string {
	driver = strings.ToLower(strings.TrimSpace(driver))
	if driver == "sqlite3" {
		return "sqlite"
	}
	if driver == "" {
		return "mysql"
	}
	return driver
}

func cleanIdentifier(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "chobits_cache"
	}
	if !identifierPattern.MatchString(value) {
		return "", fmt.Errorf("invalid SQL identifier %q", value)
	}
	return value, nil
}

func ensureContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
