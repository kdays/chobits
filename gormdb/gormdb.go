package gormdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kdays/chobits/config"
	"github.com/kdays/chobits/database"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var ErrNotFound = errors.New("gorm database connection not found")

type Option func(*Options)

type Options struct {
	Config                 *gorm.Config
	Logger                 logger.Interface
	SuppressRecordNotFound bool
}

type Manager struct {
	defaultName string
	dbs         map[string]*gorm.DB
}

func WithConfig(cfg *gorm.Config) Option {
	return func(options *Options) {
		options.Config = cfg
	}
}

func WithLogger(value logger.Interface) Option {
	return func(options *Options) {
		options.Logger = value
	}
}

func WithSuppressRecordNotFound(enabled bool) Option {
	return func(options *Options) {
		options.SuppressRecordNotFound = enabled
	}
}

func OpenManager(ctx context.Context, cfg config.Databases, opts ...Option) (*Manager, error) {
	if cfg.Default == "" {
		cfg.Default = "default"
	}
	dbs := make(map[string]*gorm.DB, len(cfg.Connections))
	for name, connection := range cfg.Connections {
		db, err := Open(ctx, connection, opts...)
		if err != nil {
			_ = closeAll(dbs)
			return nil, fmt.Errorf("open gorm database %s: %w", name, err)
		}
		dbs[name] = db
	}
	if len(dbs) == 0 {
		return nil, fmt.Errorf("no gorm database connections configured")
	}
	if _, ok := dbs[cfg.Default]; !ok {
		_ = closeAll(dbs)
		return nil, fmt.Errorf("%w: %s", ErrNotFound, cfg.Default)
	}
	return &Manager{
		defaultName: cfg.Default,
		dbs:         dbs,
	}, nil
}

func Open(ctx context.Context, cfg config.DatabaseConnection, opts ...Option) (*gorm.DB, error) {
	switch normalizedDriver(cfg.Driver) {
	case "", "mysql":
		return OpenMySQL(ctx, cfg.MySQL, opts...)
	case "sqlite":
		return OpenSQLite(ctx, cfg.SQLite, opts...)
	default:
		return nil, fmt.Errorf("unsupported gorm database driver %q", cfg.Driver)
	}
}

func OpenMySQL(ctx context.Context, cfg config.MySQL, opts ...Option) (*gorm.DB, error) {
	sqlDB, err := database.OpenMySQL(ctx, cfg)
	if err != nil {
		return nil, err
	}
	db, err := OpenSQL("mysql", sqlDB, opts...)
	if err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

func OpenSQLite(ctx context.Context, cfg config.SQLite, opts ...Option) (*gorm.DB, error) {
	return openSQLite(ctx, cfg, opts...)
}

func OpenSQL(driver string, sqlDB *sql.DB, opts ...Option) (*gorm.DB, error) {
	if sqlDB == nil {
		return nil, fmt.Errorf("sql db is nil")
	}

	var dialector gorm.Dialector
	switch normalizedDriver(driver) {
	case "", "mysql":
		dialector = gormmysql.New(gormmysql.Config{Conn: sqlDB})
	case "sqlite":
		var err error
		dialector, err = sqliteDialector(sqlDB)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported gorm database driver %q", driver)
	}
	return gorm.Open(dialector, buildConfig(opts...))
}

func (manager *Manager) Default() *gorm.DB {
	if manager == nil {
		return nil
	}
	return manager.dbs[manager.defaultName]
}

func (manager *Manager) DB(name string) (*gorm.DB, error) {
	if manager == nil {
		return nil, ErrNotFound
	}
	if name == "" {
		name = manager.defaultName
	}
	db, ok := manager.dbs[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return db, nil
}

func (manager *Manager) MustDB(name string) *gorm.DB {
	db, err := manager.DB(name)
	if err != nil {
		panic(err)
	}
	return db
}

func (manager *Manager) Names() []string {
	if manager == nil {
		return nil
	}
	names := make([]string, 0, len(manager.dbs))
	for name := range manager.dbs {
		names = append(names, name)
	}
	return names
}

func (manager *Manager) Close() error {
	if manager == nil {
		return nil
	}
	return closeAll(manager.dbs)
}

func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func closeAll(dbs map[string]*gorm.DB) error {
	var result error
	for _, db := range dbs {
		result = errors.Join(result, Close(db))
	}
	return result
}

func buildConfig(opts ...Option) *gorm.Config {
	options := Options{}
	for _, opt := range opts {
		opt(&options)
	}

	var cfg gorm.Config
	if options.Config != nil {
		cfg = *options.Config
	}
	if options.Logger != nil {
		cfg.Logger = options.Logger
	}
	if options.SuppressRecordNotFound {
		if cfg.Logger == nil {
			cfg.Logger = logger.Default
		}
		cfg.Logger = suppressNotFoundLogger{Interface: cfg.Logger}
	}
	return &cfg
}

func normalizedDriver(driver string) string {
	driver = strings.ToLower(strings.TrimSpace(driver))
	if driver == "sqlite3" {
		return "sqlite"
	}
	return driver
}

type suppressNotFoundLogger struct {
	logger.Interface
}

func (l suppressNotFoundLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	l.Interface.Trace(ctx, begin, fc, err)
}
