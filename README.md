# Chobits

Chobits is the shared Go scaffold for KDays services. It is intentionally thin:
it standardizes infrastructure and project shape, while application business
code stays inside each service.

## Packages

- `app`: HTTP server lifecycle, signal handling, graceful shutdown, background workers.
- `cli`: cobra-based default CLI with extensible commands.
- `config`: YAML loading, `.env` loading, common config structs, path/env helpers.
- `convert`: string/numeric/bool conversion helpers for request/config/migration code.
- `di`: lightweight component registry for routers, handlers, and lifecycle collection.
- `logger`: `slog` setup with level and text/json formats.
- `database`: MySQL and SQLite openers for `database/sql`.
- `kdaysuc`: KDays internal user-center client.
- `cache`: multi-store cache manager with memory, Redis, and database drivers.
- `redis`: compatibility Redis client opener; prefer `cache` for cache use cases.
- `storage`: local/upyun/external multi-disk storage manager.
- `token`: optional default login/access token service backed by `cache`.
- `response`: `{code,msg,data}` JSON envelope and pagination helpers.
- `migration`: `golang-migrate` helpers for MySQL/SQLite and file or embedded migrations.

## Intended Use

Applications should import these packages directly, for example
`github.com/kdays/chobits/cache` and `github.com/kdays/chobits/storage`,
instead of wrapping them with another project-local `cache` or `storage` package.
Project-local packages should be reserved for business-specific behavior.

Routers should receive a `*di.Container` instead of a long constructor argument
list. Register components during app bootstrap, then resolve only what each
router or handler needs.

The default CLI is cobra-based. `server` is the default command, `migrate`
provides `up`, `down`, `steps`, `force`, and `version`, and applications can add
debug/admin commands through `cli.Options.Commands` or `cli.Options.Configure`.

The cache package supports named stores with `memory`, `redis`, and `database`
drivers, one-time reads through `Take`, plus atomic counters through
`Increment`/`TTL` for simple rate-limit and retry counters. Redis uses Lua
scripts for take and increment-and-expire steps. Database-backed cache stores
receive a resolver from the app bootstrap layer, usually backed by
`database.Manager.DB`, so `cache` does not import `database` and package cycles
stay out of the scaffold.

Configuration is YAML-first. `.env` is not a second config format; it is loaded
as environment variables before YAML values are finalized. By default Chobits
applies `CHOBITS_` overrides after YAML/defaults, using the YAML path in upper
snake case. For example `database.connections.default.mysql.dsn` maps to
`CHOBITS_DATABASE_CONNECTIONS_DEFAULT_MYSQL_DSN`. Projects can use their own
prefix with `config.WithEnvPrefix("NEXT")`.

## Build Tags

Chobits keeps optional heavy integrations out of default application binaries.
The default build supports MySQL, memory/database cache, the lightweight Redis
client, local storage, and external URL storage.

Use build tags only when the app needs the optional backend:

```sh
go test ./...
go test -tags sqlite ./...
go test -tags upyun ./...
go test -tags "sqlite upyun" ./...
```

- `sqlite`: enables SQLite openers, gorm SQLite support, and the migrate SQLite
  driver. Without it, SQLite entry points return an unsupported error.
- `upyun`: enables real UPYUN object storage. Without it, UPYUN URL generation
  still works, but read/write operations return `storage.ErrUnsupported`.

Apps using Gin can usually add Gin's `nomsgpack` tag to production builds to
avoid pulling in the msgpack codec:

```sh
go build -tags nomsgpack ./cmd/server
```

When compiling an app against local Chobits changes, add a module `replace` such
as `replace github.com/kdays/chobits => ../../chobits`, or create a temporary
`go.work` containing both modules. Otherwise Go may build against the published
Chobits version instead of this checkout.

## Migration Direction

1. New projects start from `template/basic`.
2. Existing projects migrate shared infrastructure package by package.
3. Business handlers, repositories, domain models, and project-specific auth rules stay local.
