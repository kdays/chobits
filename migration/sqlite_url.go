package migration

import "strings"

func SQLiteURL(dsn string) string {
	if strings.HasPrefix(dsn, "sqlite://") {
		return dsn
	}
	return "sqlite://" + dsn
}
