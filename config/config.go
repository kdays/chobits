package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kdays/chobits/cache"
	"github.com/kdays/chobits/convert"
	"github.com/kdays/chobits/kdaysuc"
	"github.com/kdays/chobits/storage"
	"gopkg.in/yaml.v3"
)

type Defaultable interface {
	ApplyDefaults()
}

type Validatable interface {
	Validate() error
}

type Common struct {
	App      App            `yaml:"app"`
	Server   Server         `yaml:"server"`
	Database Databases      `yaml:"database"`
	MySQL    MySQL          `yaml:"mysql,omitempty"`
	SQLite   SQLite         `yaml:"sqlite,omitempty"`
	Redis    Redis          `yaml:"redis"`
	Cache    cache.Config   `yaml:"cache"`
	Log      Log            `yaml:"log"`
	Token    Token          `yaml:"token"`
	Storage  storage.Config `yaml:"storage"`
	KDaysUC  kdaysuc.Config `yaml:"kdays_uc"`
	Features Features       `yaml:"features"`
}

type App struct {
	Name    string `yaml:"name"`
	Env     string `yaml:"env"`
	Debug   bool   `yaml:"debug"`
	DataDir string `yaml:"data_dir"`
}

type Server struct {
	Addr                     string   `yaml:"addr"`
	AllowedOrigins           []string `yaml:"allowed_origins"`
	ReadHeaderTimeoutSeconds int      `yaml:"read_header_timeout_seconds"`
	ShutdownTimeoutSeconds   int      `yaml:"shutdown_timeout_seconds"`
}

type Databases struct {
	Default     string                        `yaml:"default"`
	Driver      string                        `yaml:"driver,omitempty"`
	Connections map[string]DatabaseConnection `yaml:"connections"`
}

type DatabaseConnection struct {
	Driver string `yaml:"driver"`
	MySQL  MySQL  `yaml:"mysql,omitempty"`
	SQLite SQLite `yaml:"sqlite,omitempty"`
}

type MySQL struct {
	DSN                    string `yaml:"dsn"`
	Host                   string `yaml:"host"`
	Port                   int    `yaml:"port"`
	User                   string `yaml:"user"`
	Password               string `yaml:"password"`
	Database               string `yaml:"database"`
	DBName                 string `yaml:"dbname"`
	Charset                string `yaml:"charset"`
	MaxOpenConns           int    `yaml:"max_open_conns"`
	MaxIdleConns           int    `yaml:"max_idle_conns"`
	ConnMaxLifetimeSeconds int    `yaml:"conn_max_lifetime_seconds"`
}

type SQLite struct {
	DSN                    string `yaml:"dsn"`
	Path                   string `yaml:"path"`
	MaxOpenConns           int    `yaml:"max_open_conns"`
	MaxIdleConns           int    `yaml:"max_idle_conns"`
	ConnMaxLifetimeSeconds int    `yaml:"conn_max_lifetime_seconds"`
}

type Redis struct {
	Addr     string `yaml:"addr"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	Prefix   string `yaml:"prefix"`
	Required bool   `yaml:"required"`
}

type Log struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type Token struct {
	Prefix     string `yaml:"prefix"`
	TTLSeconds int    `yaml:"ttl_seconds"`
}

type Features map[string]bool

type LoadOptions struct {
	DotEnvPaths      []string
	EnvPrefix        string
	ApplyEnvOverride bool
	SearchNames      []string
	SearchDirs       []string
}

type LoadOption func(*LoadOptions)

func WithDotEnv(paths ...string) LoadOption {
	return func(options *LoadOptions) {
		options.DotEnvPaths = append(options.DotEnvPaths, paths...)
	}
}

func WithSearchNames(names ...string) LoadOption {
	return func(options *LoadOptions) {
		options.SearchNames = append(options.SearchNames, names...)
	}
}

func WithSearchDirs(dirs ...string) LoadOption {
	return func(options *LoadOptions) {
		options.SearchDirs = append(options.SearchDirs, dirs...)
	}
}

func WithEnvPrefix(prefix string) LoadOption {
	return func(options *LoadOptions) {
		options.EnvPrefix = prefix
		options.ApplyEnvOverride = true
	}
}

func WithEnvOverride(enabled bool) LoadOption {
	return func(options *LoadOptions) {
		options.ApplyEnvOverride = enabled
	}
}

func LoadYAML(path string, target any, opts ...LoadOption) (string, error) {
	if target == nil {
		return "", fmt.Errorf("config target is nil")
	}

	options := LoadOptions{
		DotEnvPaths:      []string{".env"},
		EnvPrefix:        "CHOBITS",
		ApplyEnvOverride: true,
		SearchNames:      []string{"config.yaml", "config.yml"},
		SearchDirs:       []string{".", "./config"},
	}
	for _, opt := range opts {
		opt(&options)
	}
	_ = TryLoadDotEnv(options.DotEnvPaths...)

	used, err := FindFile(path, options.SearchNames, options.SearchDirs)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(used)
	if err != nil {
		return "", err
	}
	if err := yaml.Unmarshal(data, target); err != nil {
		return "", fmt.Errorf("unmarshal config %s: %w", used, err)
	}
	if options.ApplyEnvOverride {
		if err := ApplyEnv(target, options.EnvPrefix); err != nil {
			return "", err
		}
	}
	if value, ok := target.(Defaultable); ok {
		value.ApplyDefaults()
	}
	if options.ApplyEnvOverride {
		if err := ApplyEnv(target, options.EnvPrefix); err != nil {
			return "", err
		}
	}
	if value, ok := target.(Validatable); ok {
		if err := value.Validate(); err != nil {
			return "", err
		}
	}
	return used, nil
}

func FindFile(explicit string, names []string, dirs []string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", err
		}
		return explicit, nil
	}
	for _, dir := range dirs {
		for _, name := range names {
			candidate := filepath.Join(dir, name)
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("config file not found")
}

func (cfg *Common) ApplyDefaults() {
	if cfg.App.Env == "" {
		cfg.App.Env = "dev"
	}
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = ":8080"
	}
	if cfg.Server.ReadHeaderTimeoutSeconds == 0 {
		cfg.Server.ReadHeaderTimeoutSeconds = 5
	}
	if cfg.Server.ShutdownTimeoutSeconds == 0 {
		cfg.Server.ShutdownTimeoutSeconds = 10
	}
	cfg.applyDatabaseDefaults()
	if cfg.Redis.Addr == "" {
		cfg.Redis.Addr = "127.0.0.1:6379"
	}
	cfg.Cache.ApplyDefaults()
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Token.Prefix == "" {
		cfg.Token.Prefix = "cbt_"
	}
	if cfg.Token.TTLSeconds == 0 {
		cfg.Token.TTLSeconds = int((120 * time.Hour).Seconds())
	}
	if cfg.Storage.MaxUploadSize == 0 {
		cfg.Storage.MaxUploadSize = 2 * 1024 * 1024
	}
	if cfg.KDaysUC.TimeoutSeconds == 0 {
		cfg.KDaysUC.TimeoutSeconds = 10
	}
}

func (cfg *Common) applyDatabaseDefaults() {
	if cfg.Database.Default == "" {
		cfg.Database.Default = "default"
	}
	if cfg.Database.Connections == nil {
		cfg.Database.Connections = map[string]DatabaseConnection{}
	}
	if len(cfg.Database.Connections) == 0 {
		driver := cfg.Database.Driver
		if driver == "" {
			driver = "mysql"
		}
		cfg.Database.Connections[cfg.Database.Default] = DatabaseConnection{
			Driver: driver,
			MySQL:  cfg.MySQL,
			SQLite: cfg.SQLite,
		}
	}
	for name, connection := range cfg.Database.Connections {
		if connection.Driver == "" {
			connection.Driver = "mysql"
		}
		applyMySQLDefaults(&connection.MySQL)
		applySQLiteDefaults(&connection.SQLite)
		cfg.Database.Connections[name] = connection
	}
}

func (cfg Databases) Connection(name string) (DatabaseConnection, error) {
	common := Common{Database: cfg}
	common.ApplyDefaults()
	dbs := common.Database
	if strings.TrimSpace(name) == "" {
		name = dbs.Default
	}
	connection, ok := dbs.Connections[name]
	if !ok {
		return DatabaseConnection{}, fmt.Errorf("database connection %q not found", name)
	}
	return connection, nil
}

func (cfg Databases) DriverName(name string) (string, error) {
	connection, err := cfg.Connection(name)
	if err != nil {
		return "", err
	}
	driver := strings.ToLower(strings.TrimSpace(connection.Driver))
	if driver == "" {
		driver = "mysql"
	}
	if driver == "sqlite3" {
		driver = "sqlite"
	}
	return driver, nil
}

func applyMySQLDefaults(cfg *MySQL) {
	if cfg.Port == 0 {
		cfg.Port = 3306
	}
	if cfg.Charset == "" {
		cfg.Charset = "utf8mb4"
	}
	if cfg.MaxOpenConns == 0 {
		cfg.MaxOpenConns = 25
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = 10
	}
	if cfg.ConnMaxLifetimeSeconds == 0 {
		cfg.ConnMaxLifetimeSeconds = 300
	}
}

func applySQLiteDefaults(cfg *SQLite) {
	if cfg.Path == "" && cfg.DSN == "" {
		cfg.Path = "./data/app.db"
	}
	if cfg.MaxOpenConns == 0 {
		cfg.MaxOpenConns = 1
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = 1
	}
	if cfg.ConnMaxLifetimeSeconds == 0 {
		cfg.ConnMaxLifetimeSeconds = 300
	}
}

func (cfg Server) ReadHeaderTimeout() time.Duration {
	return time.Duration(cfg.ReadHeaderTimeoutSeconds) * time.Second
}

func (cfg Server) ShutdownTimeout() time.Duration {
	return time.Duration(cfg.ShutdownTimeoutSeconds) * time.Second
}

func (cfg Token) TTL() time.Duration {
	return time.Duration(cfg.TTLSeconds) * time.Second
}

func (cfg MySQL) DSNString() (string, error) {
	if strings.TrimSpace(cfg.DSN) != "" {
		return cfg.DSN, nil
	}
	name := cfg.Database
	if name == "" {
		name = cfg.DBName
	}
	if cfg.Host == "" || cfg.User == "" || name == "" {
		return "", fmt.Errorf("mysql dsn is required, or host/user/database must be set")
	}
	charset := cfg.Charset
	if charset == "" {
		charset = "utf8mb4"
	}
	port := cfg.Port
	if port == 0 {
		port = 3306
	}
	params := url.Values{}
	params.Set("charset", charset)
	params.Set("parseTime", "true")
	params.Set("loc", "Local")
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s", cfg.User, cfg.Password, cfg.Host, port, name, params.Encode()), nil
}

func (cfg SQLite) DSNString() (string, error) {
	if strings.TrimSpace(cfg.DSN) != "" {
		return cfg.DSN, nil
	}
	if strings.TrimSpace(cfg.Path) == "" {
		return "", fmt.Errorf("sqlite dsn or path is required")
	}
	return cfg.Path, nil
}

func ResolvePath(baseDir, value string) string {
	if value == "" || filepath.IsAbs(value) {
		return value
	}
	if baseDir == "" {
		baseDir = "."
	}
	return filepath.Clean(filepath.Join(baseDir, value))
}

func Env(key string) string {
	return os.Getenv(key)
}

func EnvOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func EnvIntOrDefault(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := convert.Int(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func EnvBoolOrDefault(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := convert.Bool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
