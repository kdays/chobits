//go:build sqlite

package gormdb

import (
	"context"
	"database/sql"

	"github.com/kdays/chobits/config"
	"github.com/kdays/chobits/database"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openSQLite(ctx context.Context, cfg config.SQLite, opts ...Option) (*gorm.DB, error) {
	sqlDB, err := database.OpenSQLite(ctx, cfg)
	if err != nil {
		return nil, err
	}
	db, err := OpenSQL("sqlite", sqlDB, opts...)
	if err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

func sqliteDialector(sqlDB *sql.DB) (gorm.Dialector, error) {
	return gormsqlite.New(gormsqlite.Config{Conn: sqlDB}), nil
}
