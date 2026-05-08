package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommonDatabaseDefaultsBuildDefaultConnection(t *testing.T) {
	t.Parallel()

	cfg := Common{
		MySQL: MySQL{
			DSN: "user:pass@tcp(127.0.0.1:3306)/app?parseTime=true",
		},
	}
	cfg.ApplyDefaults()

	if cfg.Database.Default != "default" {
		t.Fatalf("default = %q, want default", cfg.Database.Default)
	}
	connection, ok := cfg.Database.Connections["default"]
	if !ok {
		t.Fatalf("default connection missing")
	}
	if connection.Driver != "mysql" {
		t.Fatalf("driver = %q, want mysql", connection.Driver)
	}
	if connection.MySQL.DSN != cfg.MySQL.DSN {
		t.Fatalf("dsn = %q, want %q", connection.MySQL.DSN, cfg.MySQL.DSN)
	}
}

func TestDatabasesConnectionAndDriverName(t *testing.T) {
	t.Parallel()

	cfg := Databases{
		Default: "primary",
		Connections: map[string]DatabaseConnection{
			"primary": {
				Driver: "mysql",
				MySQL:  MySQL{DSN: "mysql-dsn"},
			},
			"local": {
				Driver: "sqlite3",
				SQLite: SQLite{
					Path: "local.db",
				},
			},
		},
	}

	connection, err := cfg.Connection("")
	if err != nil {
		t.Fatalf("Connection() error = %v", err)
	}
	if connection.MySQL.DSN != "mysql-dsn" {
		t.Fatalf("default connection dsn = %q, want mysql-dsn", connection.MySQL.DSN)
	}

	driver, err := cfg.DriverName("local")
	if err != nil {
		t.Fatalf("DriverName() error = %v", err)
	}
	if driver != "sqlite" {
		t.Fatalf("driver = %q, want sqlite", driver)
	}
}

func TestDatabasesConnectionRejectsMissingDefault(t *testing.T) {
	t.Parallel()

	cfg := Databases{
		Default: "missing",
		Connections: map[string]DatabaseConnection{
			"default": {
				Driver: "mysql",
				MySQL:  MySQL{DSN: "mysql-dsn"},
			},
		},
	}

	if _, err := cfg.Connection(""); err == nil {
		t.Fatalf("Connection() error = nil, want missing default error")
	}
}

func TestCommonCacheDefaultsBuildDefaultStore(t *testing.T) {
	t.Parallel()

	var cfg Common
	cfg.ApplyDefaults()

	if cfg.Cache.DefaultStore != "default" {
		t.Fatalf("default store = %q, want default", cfg.Cache.DefaultStore)
	}
	store, ok := cfg.Cache.Stores["default"]
	if !ok {
		t.Fatalf("default cache store missing")
	}
	if store.Driver != "memory" {
		t.Fatalf("cache driver = %q, want memory", store.Driver)
	}
}

func TestLoadYAMLAppliesDotEnvOverrides(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(configPath, []byte(`
app:
  name: demo
server:
  addr: ":8080"
database:
  default: default
  connections:
    default:
      driver: mysql
      mysql:
        dsn: yaml-dsn
cache:
  default_store: default
  stores:
    default:
      driver: redis
      redis:
        addr: 127.0.0.1:6379
        prefix: yaml-prefix
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envPath, []byte(`
TESTAPP_SERVER_ADDR=:9090
TESTAPP_DATABASE_CONNECTIONS_DEFAULT_MYSQL_DSN=env-dsn
TESTAPP_CACHE_STORES_DEFAULT_REDIS_PREFIX=env-prefix
`), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Common
	if _, err := LoadYAML(configPath, &cfg, WithDotEnv(envPath), WithEnvPrefix("TESTAPP")); err != nil {
		t.Fatalf("LoadYAML() error = %v", err)
	}

	if cfg.Server.Addr != ":9090" {
		t.Fatalf("addr = %q, want :9090", cfg.Server.Addr)
	}
	if cfg.Database.Connections["default"].MySQL.DSN != "env-dsn" {
		t.Fatalf("dsn = %q, want env-dsn", cfg.Database.Connections["default"].MySQL.DSN)
	}
	if cfg.Cache.Stores["default"].Redis.Prefix != "env-prefix" {
		t.Fatalf("cache prefix = %q, want env-prefix", cfg.Cache.Stores["default"].Redis.Prefix)
	}
}
