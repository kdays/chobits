# Basic Chobits App Template

Copy this directory into a new project and replace:

- `{{MODULE}}` with the project module path.
- `{{APP_NAME}}` with the binary/application name.

The template keeps `cmd/server/main.go` small. Application-specific dependencies
are assembled in `internal/app`, while shared infrastructure comes directly from
`github.com/kdays/chobits`.

The generated app uses the Chobits DI container. Register application services in
`internal/app`, then resolve them from `internal/router` or handlers with
`di.Resolve` / `di.MustResolve`.

The CLI is backed by cobra. Add project-specific debug or admin commands through
`cli.Options.Commands` or `cli.Options.Configure`.

Chobits keeps SQLite and UPYUN out of default binaries. Build with `-tags sqlite`
only if the generated app needs SQLite support, and with `-tags upyun` only if it
needs the real UPYUN storage backend. Apps using Gin can usually add Gin's
`nomsgpack` tag to production builds to avoid the msgpack codec.
