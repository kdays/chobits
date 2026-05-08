package cache

import "errors"

var (
	ErrMiss          = errors.New("cache miss")
	ErrUnavailable   = errors.New("cache unavailable")
	ErrStoreNotFound = errors.New("cache store not found")
	ErrDatabaseUnset = errors.New("cache database is not configured")
)
