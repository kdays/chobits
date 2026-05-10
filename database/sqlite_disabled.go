//go:build !sqlite

package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kdays/chobits/config"
)

func OpenSQLite(context.Context, config.SQLite) (*sql.DB, error) {
	return nil, fmt.Errorf("sqlite support is disabled; rebuild with -tags sqlite")
}
