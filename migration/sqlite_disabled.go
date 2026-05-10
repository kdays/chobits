//go:build !sqlite

package migration

import (
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
)

func NewSQLiteFile(string, string) (*migrate.Migrate, error) {
	return nil, sqliteDisabledError()
}

func NewSQLiteEmbed(string, fs.FS, string) (*migrate.Migrate, error) {
	return nil, sqliteDisabledError()
}

func sqliteDisabledError() error {
	return fmt.Errorf("sqlite support is disabled; rebuild with -tags sqlite")
}
