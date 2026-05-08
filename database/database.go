package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/kdays/chobits/config"
)

var ErrNotFound = errors.New("database connection not found")

type Manager struct {
	defaultName string
	dbs         map[string]*sql.DB
}

func OpenManager(ctx context.Context, cfg config.Databases) (*Manager, error) {
	if cfg.Default == "" {
		cfg.Default = "default"
	}
	dbs := make(map[string]*sql.DB, len(cfg.Connections))
	for name, connection := range cfg.Connections {
		db, err := Open(ctx, connection)
		if err != nil {
			closeAll(dbs)
			return nil, fmt.Errorf("open database %s: %w", name, err)
		}
		dbs[name] = db
	}
	if len(dbs) == 0 {
		return nil, fmt.Errorf("no database connections configured")
	}
	if _, ok := dbs[cfg.Default]; !ok {
		closeAll(dbs)
		return nil, fmt.Errorf("%w: %s", ErrNotFound, cfg.Default)
	}
	return &Manager{
		defaultName: cfg.Default,
		dbs:         dbs,
	}, nil
}

func Open(ctx context.Context, cfg config.DatabaseConnection) (*sql.DB, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Driver)) {
	case "", "mysql":
		return OpenMySQL(ctx, cfg.MySQL)
	case "sqlite", "sqlite3":
		return OpenSQLite(ctx, cfg.SQLite)
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.Driver)
	}
}

func (manager *Manager) Default() *sql.DB {
	if manager == nil {
		return nil
	}
	return manager.dbs[manager.defaultName]
}

func (manager *Manager) DB(name string) (*sql.DB, error) {
	if manager == nil {
		return nil, ErrNotFound
	}
	if name == "" {
		name = manager.defaultName
	}
	db, ok := manager.dbs[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return db, nil
}

func (manager *Manager) MustDB(name string) *sql.DB {
	db, err := manager.DB(name)
	if err != nil {
		panic(err)
	}
	return db
}

func (manager *Manager) Names() []string {
	if manager == nil {
		return nil
	}
	names := make([]string, 0, len(manager.dbs))
	for name := range manager.dbs {
		names = append(names, name)
	}
	return names
}

func (manager *Manager) Close() error {
	if manager == nil {
		return nil
	}
	return closeAll(manager.dbs)
}

func closeAll(dbs map[string]*sql.DB) error {
	var result error
	for _, db := range dbs {
		if db == nil {
			continue
		}
		result = errors.Join(result, db.Close())
	}
	return result
}
