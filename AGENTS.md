# Chobits Agent Notes

## Build And Test

Chobits is a Go module. When editing the framework itself, run:

- `go test ./...`
- `go test -tags sqlite ./...` after changing SQLite, database, gormdb, or migration code.
- `go test -tags upyun ./...` after changing storage or UPYUN code.

Default builds must stay small. Do not put optional heavy dependencies in files
compiled by default.

## Optional Build Tags

- `sqlite`: enables SQLite support in `database`, `gormdb`, and `migration`.
  Without this tag, SQLite entry points return an unsupported error and
  `modernc.org/sqlite`, `gorm.io/driver/sqlite`, and the migrate SQLite driver
  should not enter default application binaries.
- `upyun`: enables the real UPYUN storage backend. Without this tag, UPYUN disk
  URL generation remains available, but read/write operations return
  `storage.ErrUnsupported`.

Apps using Gin should normally build with Gin's `nomsgpack` tag unless they
explicitly need msgpack binding.

## Using Local Chobits From An App

When an app in a sibling repository needs to compile against local Chobits
changes, make Go resolve this checkout explicitly. Use either:

- a `replace github.com/kdays/chobits => ../../chobits` directive in the app
  module, adjusted for that app's relative path; or
- a temporary `go.work` that lists both the app module and this Chobits module.

If the app does not use a local `replace` or `go.work`, `go build` may compile
against the published `github.com/kdays/chobits` version instead of these local
framework changes.
