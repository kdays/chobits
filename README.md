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

## Migration Direction

1. New projects start from `template/basic`.
2. Existing projects migrate shared infrastructure package by package.
3. Business handlers, repositories, domain models, and project-specific auth rules stay local.
