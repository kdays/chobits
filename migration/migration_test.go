package migration

import (
	"database/sql"
	"embed"
	"errors"
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "modernc.org/sqlite"
)

//go:embed testdata/migrations/*.sql
var testMigrations embed.FS

func TestSQLiteEmbedMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	m, err := NewSQLiteEmbed(dbPath, testMigrations, "testdata/migrations")
	if err != nil {
		t.Fatalf("new sqlite embedded migration: %v", err)
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := Up(m); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	version, dirty, err := Version(m)
	if err != nil {
		t.Fatalf("migration version: %v", err)
	}
	if version != 1 || dirty {
		t.Fatalf("unexpected migration version: version=%d dirty=%t", version, dirty)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec("INSERT INTO widgets (name) VALUES ('first')"); err != nil {
		t.Fatalf("insert migrated table: %v", err)
	}

	if err := Down(m); err != nil {
		t.Fatalf("migrate down: %v", err)
	}
	_, dirty, err = Version(m)
	if !errors.Is(err, migrate.ErrNilVersion) {
		t.Fatalf("expected nil version after down, got version dirty=%t err=%v", dirty, err)
	}
}

func TestNewFileRejectsUnsupportedDriver(t *testing.T) {
	_, err := NewFile("oracle", "dsn", "migrations")
	if err == nil {
		t.Fatalf("expected unsupported driver error")
	}
}
