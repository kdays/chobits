package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/kdays/chobits/app"
	"github.com/kdays/chobits/convert"
	"github.com/kdays/chobits/migration"
	"github.com/spf13/cobra"
)

type Context struct {
	ConfigPath string
	Stdout     io.Writer
	Stderr     io.Writer
	Logger     *slog.Logger
}

type ServerFunc func(context.Context, *Context) (*http.Server, app.Options, error)

type MigrateFunc func(context.Context, *Context) (*migrate.Migrate, error)

type Options struct {
	Name              string
	Short             string
	Args              []string
	DefaultConfigPath string
	Logger            *slog.Logger
	Stdout            io.Writer
	Stderr            io.Writer
	VersionText       func() string
	Server            ServerFunc
	Migrate           MigrateFunc
	Commands          []*cobra.Command
	Configure         func(*cobra.Command, *Context)
}

func Execute(ctx context.Context, options Options) error {
	root, _ := NewRoot(ctx, options)
	return root.Execute()
}

func ExecuteOrExit(ctx context.Context, options Options) {
	root, c := NewRoot(ctx, options)
	if err := root.Execute(); err != nil {
		c.Logger.Error("command failed", slog.Any("error", err))
		os.Exit(1)
	}
}

func NewRoot(ctx context.Context, options Options) (*cobra.Command, *Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if options.Stdout == nil {
		options.Stdout = os.Stdout
	}
	if options.Stderr == nil {
		options.Stderr = os.Stderr
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	if options.DefaultConfigPath == "" {
		options.DefaultConfigPath = "config.yaml"
	}
	if options.Name == "" {
		options.Name = "server"
	}

	c := &Context{
		ConfigPath: options.DefaultConfigPath,
		Stdout:     options.Stdout,
		Stderr:     options.Stderr,
		Logger:     options.Logger,
	}

	root := &cobra.Command{
		Use:           options.Name,
		Short:         options.Short,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(ctx, c, options.Server)
		},
	}
	root.SetOut(options.Stdout)
	root.SetErr(options.Stderr)
	if options.VersionText != nil {
		root.Version = strings.TrimSpace(options.VersionText())
		root.SetVersionTemplate("{{.Version}}\n")
	}
	if options.Args != nil {
		root.SetArgs(options.Args)
	}
	root.PersistentFlags().StringVarP(&c.ConfigPath, "config", "c", options.DefaultConfigPath, "config file path")

	root.AddCommand(serverCommand(ctx, c, options.Server))
	if options.Migrate != nil {
		root.AddCommand(migrateCommand(ctx, c, options.Migrate))
	}
	if options.VersionText != nil {
		root.AddCommand(versionCommand(c, options.VersionText))
	}
	for _, command := range options.Commands {
		if command != nil {
			root.AddCommand(command)
		}
	}
	if options.Configure != nil {
		options.Configure(root, c)
	}

	return root, c
}

func Command(ctx context.Context, c *Context, use string, short string, run func(context.Context, *Context, *cobra.Command, []string) error) *cobra.Command {
	if ctx == nil {
		ctx = context.Background()
	}
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(ctx, c, cmd, args)
		},
	}
}

func serverCommand(ctx context.Context, c *Context, server ServerFunc) *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "start HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(ctx, c, server)
		},
	}
}

func runServer(ctx context.Context, c *Context, server ServerFunc) error {
	if server == nil {
		return fmt.Errorf("server command is not configured")
	}
	srv, runOptions, err := server(ctx, c)
	if err != nil {
		return err
	}
	if runOptions.Logger == nil {
		runOptions.Logger = c.Logger
	}
	return app.RunHTTP(ctx, srv, runOptions)
}

func versionCommand(c *Context, versionText func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "print version",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _ = io.WriteString(c.Stdout, versionText())
			return nil
		},
	}
}

func migrateCommand(ctx context.Context, c *Context, factory MigrateFunc) *cobra.Command {
	command := &cobra.Command{
		Use:   "migrate",
		Short: "run database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return migrateUp(ctx, c, factory)
		},
	}
	command.AddCommand(&cobra.Command{
		Use:   "up",
		Short: "apply pending migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return migrateUp(ctx, c, factory)
		},
	})
	command.AddCommand(&cobra.Command{
		Use:   "down",
		Short: "roll back all migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := factory(ctx, c)
			if err != nil {
				return err
			}
			defer closeMigrate(m)
			return migration.Down(m)
		},
	})
	command.AddCommand(&cobra.Command{
		Use:   "steps <n>",
		Short: "migrate n steps; negative values roll back",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := convert.Int(args[0])
			if err != nil {
				return fmt.Errorf("invalid migration steps %q: %w", args[0], err)
			}
			m, err := factory(ctx, c)
			if err != nil {
				return err
			}
			defer closeMigrate(m)
			return migration.Steps(m, n)
		},
	})
	command.AddCommand(&cobra.Command{
		Use:   "force <version>",
		Short: "force migration version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			version, err := convert.Int(args[0])
			if err != nil {
				return fmt.Errorf("invalid migration version %q: %w", args[0], err)
			}
			m, err := factory(ctx, c)
			if err != nil {
				return err
			}
			defer closeMigrate(m)
			return m.Force(version)
		},
	})
	command.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "print migration version",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := factory(ctx, c)
			if err != nil {
				return err
			}
			defer closeMigrate(m)
			version, dirty, err := migration.Version(m)
			if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
				return err
			}
			if errors.Is(err, migrate.ErrNilVersion) {
				version = 0
				dirty = false
			}
			_, _ = fmt.Fprintf(c.Stdout, "version=%d dirty=%t\n", version, dirty)
			return nil
		},
	})
	return command
}

func migrateUp(ctx context.Context, c *Context, factory MigrateFunc) error {
	m, err := factory(ctx, c)
	if err != nil {
		return err
	}
	defer closeMigrate(m)
	return migration.Up(m)
}

func closeMigrate(m *migrate.Migrate) {
	if m == nil {
		return
	}
	_, _ = m.Close()
}
