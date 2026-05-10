//go:build sqlite

package migration

import (
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite"
)

func NewSQLiteFile(dsn string, migrationsDir string) (*migrate.Migrate, error) {
	sourceURL := "file://" + migrationsDir
	return newSQLite(sourceURL, dsn)
}

func NewSQLiteEmbed(dsn string, files fs.FS, dir string) (*migrate.Migrate, error) {
	return newEmbed(files, dir, SQLiteURL(dsn))
}

func newSQLite(sourceURL string, dsn string) (*migrate.Migrate, error) {
	m, err := migrate.New(sourceURL, SQLiteURL(dsn))
	if err != nil {
		return nil, fmt.Errorf("create migrate instance: %w", err)
	}
	return m, nil
}
