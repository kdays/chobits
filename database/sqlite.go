//go:build sqlite

package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kdays/chobits/config"
	_ "modernc.org/sqlite"
)

func OpenSQLite(ctx context.Context, cfg config.SQLite) (*sql.DB, error) {
	dsn, err := cfg.DSNString()
	if err != nil {
		return nil, err
	}
	if cfg.DSN == "" && cfg.Path != "" && !isInMemorySQLitePath(cfg.Path) {
		if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	maxOpen := cfg.MaxOpenConns
	if maxOpen == 0 {
		maxOpen = 1
	}
	maxIdle := cfg.MaxIdleConns
	if maxIdle == 0 {
		maxIdle = 1
	}
	lifetime := time.Duration(cfg.ConnMaxLifetimeSeconds) * time.Second
	if lifetime <= 0 {
		lifetime = 5 * time.Minute
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(lifetime)

	if ctx == nil {
		ctx = context.Background()
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return db, nil
}

func isInMemorySQLitePath(value string) bool {
	value = strings.TrimSpace(value)
	return value == ":memory:" || strings.HasPrefix(value, "file::memory:")
}
