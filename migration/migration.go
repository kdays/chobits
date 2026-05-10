package migration

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	iofsdriver "github.com/golang-migrate/migrate/v4/source/iofs"
)

func NewFile(driver string, dsn string, migrationsDir string) (*migrate.Migrate, error) {
	sourceURL := "file://" + migrationsDir
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", "mysql":
		return newMySQL(sourceURL, dsn)
	case "sqlite", "sqlite3":
		return NewSQLiteFile(dsn, migrationsDir)
	default:
		return nil, fmt.Errorf("unsupported migration database driver %q", driver)
	}
}

func NewEmbed(driver string, dsn string, files fs.FS, dir string) (*migrate.Migrate, error) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", "mysql":
		return NewMySQLEmbed(dsn, files, dir)
	case "sqlite", "sqlite3":
		return NewSQLiteEmbed(dsn, files, dir)
	default:
		return nil, fmt.Errorf("unsupported migration database driver %q", driver)
	}
}

func NewMySQLFile(dsn string, migrationsDir string) (*migrate.Migrate, error) {
	sourceURL := "file://" + migrationsDir
	return newMySQL(sourceURL, dsn)
}

func NewMySQLEmbed(dsn string, files fs.FS, dir string) (*migrate.Migrate, error) {
	return newEmbed(files, dir, mysqlURL(dsn))
}

func Up(m *migrate.Migrate) error {
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

func Down(m *migrate.Migrate) error {
	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

func Steps(m *migrate.Migrate, n int) error {
	if err := m.Steps(n); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate steps(%d): %w", n, err)
	}
	return nil
}

func Version(m *migrate.Migrate) (uint, bool, error) {
	return m.Version()
}

func newMySQL(sourceURL string, dsn string) (*migrate.Migrate, error) {
	m, err := migrate.New(sourceURL, mysqlURL(dsn))
	if err != nil {
		return nil, fmt.Errorf("create migrate instance: %w", err)
	}
	return m, nil
}

func newEmbed(files fs.FS, dir string, databaseURL string) (*migrate.Migrate, error) {
	sourceDriver, err := iofsdriver.New(files, dir)
	if err != nil {
		return nil, fmt.Errorf("create embedded migrate source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create migrate instance: %w", err)
	}
	return m, nil
}

func mysqlURL(dsn string) string {
	if strings.HasPrefix(dsn, "mysql://") {
		return dsn
	}
	return "mysql://" + dsn
}
