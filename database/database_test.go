package database

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/kdays/chobits/config"
)

func TestOpenManagerSQLite(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "app.db")
	manager, err := OpenManager(context.Background(), config.Databases{
		Default: "default",
		Connections: map[string]config.DatabaseConnection{
			"default": {
				Driver: "sqlite",
				SQLite: config.SQLite{
					Path: dbPath,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("OpenManager() error = %v", err)
	}
	defer manager.Close()

	if manager.Default() == nil {
		t.Fatalf("default db is nil")
	}
	if _, err := manager.Default().Exec(`CREATE TABLE ping (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("sqlite exec error = %v", err)
	}
}

func TestOpenManagerRejectsMissingDefault(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "app.db")
	_, err := OpenManager(context.Background(), config.Databases{
		Default: "missing",
		Connections: map[string]config.DatabaseConnection{
			"default": {
				Driver: "sqlite",
				SQLite: config.SQLite{Path: dbPath},
			},
		},
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("OpenManager() error = %v, want ErrNotFound", err)
	}
}
