//go:build !sqlite

package gormdb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kdays/chobits/config"
	"gorm.io/gorm"
)

func openSQLite(context.Context, config.SQLite, ...Option) (*gorm.DB, error) {
	return nil, fmt.Errorf("sqlite support is disabled; rebuild with -tags sqlite")
}

func sqliteDialector(*sql.DB) (gorm.Dialector, error) {
	return nil, fmt.Errorf("sqlite support is disabled; rebuild with -tags sqlite")
}
