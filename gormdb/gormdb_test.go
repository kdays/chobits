package gormdb

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/kdays/chobits/config"
	"gorm.io/gorm/logger"
)

type testRecord struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

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
	}, WithLogger(logger.Default.LogMode(logger.Silent)), WithSuppressRecordNotFound(true))
	if err != nil {
		t.Fatalf("OpenManager() error = %v", err)
	}
	defer manager.Close()

	db := manager.Default()
	if db == nil {
		t.Fatalf("default db is nil")
	}
	if err := db.AutoMigrate(&testRecord{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	if err := db.Create(&testRecord{Name: "kiruya"}).Error; err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	var got testRecord
	if err := db.First(&got, "name = ?", "kiruya").Error; err != nil {
		t.Fatalf("First() error = %v", err)
	}
	if got.Name != "kiruya" {
		t.Fatalf("got name %q, want kiruya", got.Name)
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
	}, WithLogger(logger.Default.LogMode(logger.Silent)))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("OpenManager() error = %v, want ErrNotFound", err)
	}
}
