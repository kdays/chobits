package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

type Options struct {
	Level     string
	Format    string
	Env       string
	AddSource bool
}

func New(output io.Writer, options Options) (*slog.Logger, error) {
	if output == nil {
		output = os.Stderr
	}
	level, err := ParseLevel(options.Level)
	if err != nil {
		return nil, err
	}
	handlerOptions := &slog.HandlerOptions{
		Level:     level,
		AddSource: options.AddSource,
	}
	format := strings.ToLower(strings.TrimSpace(options.Format))
	if format == "" && strings.TrimSpace(options.Env) != "dev" {
		format = "json"
	}
	if format == "json" {
		return slog.New(slog.NewJSONHandler(output, handlerOptions)), nil
	}
	return slog.New(slog.NewTextHandler(output, handlerOptions)), nil
}

func Must(output io.Writer, options Options) *slog.Logger {
	logger, err := New(output, options)
	if err != nil {
		panic(err)
	}
	return logger
}

func ParseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level %q", value)
	}
}
