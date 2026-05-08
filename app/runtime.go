package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Background interface {
	Start(context.Context) error
}

type Stopper interface {
	Stop(context.Context) error
}

type Waiter interface {
	Wait()
}

type Closer interface {
	Close() error
}

type Options struct {
	Logger          *slog.Logger
	ShutdownTimeout time.Duration
	Signals         []os.Signal
	Backgrounds     []any
	Closers         []Closer
}

func RunHTTP(ctx context.Context, srv *http.Server, options Options) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if srv == nil {
		return errors.New("http server is nil")
	}

	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}

	shutdownTimeout := options.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = 10 * time.Second
	}

	signals := options.Signals
	if len(signals) == 0 {
		signals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	}
	ctx, stopNotify := signal.NotifyContext(ctx, signals...)
	defer stopNotify()

	started := make([]any, 0, len(options.Backgrounds))
	for _, service := range options.Backgrounds {
		if service == nil {
			continue
		}
		if starter, ok := service.(Background); ok {
			if err := starter.Start(ctx); err != nil {
				stopBackgrounds(context.Background(), logger, started, shutdownTimeout)
				closeResources(logger, options.Closers)
				return err
			}
		}
		started = append(started, service)
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server starting", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	var serveErr error
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case serveErr = <-errCh:
		if serveErr != nil {
			logger.Error("http server stopped unexpectedly", slog.Any("error", serveErr))
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	var result error
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		result = errors.Join(result, err)
	}
	result = errors.Join(result, stopBackgrounds(context.Background(), logger, started, shutdownTimeout))
	result = errors.Join(result, closeResources(logger, options.Closers))
	if serveErr != nil {
		result = errors.Join(serveErr, result)
	}
	if result == nil {
		logger.Info("http server exited")
	}
	return result
}

func stopBackgrounds(ctx context.Context, logger *slog.Logger, services []any, timeout time.Duration) error {
	var result error
	for i := len(services) - 1; i >= 0; i-- {
		service := services[i]
		if service == nil {
			continue
		}
		stopCtx, cancel := context.WithTimeout(ctx, timeout)
		if stopper, ok := service.(Stopper); ok {
			if err := stopper.Stop(stopCtx); err != nil {
				logger.Error("stop background service", slog.Any("error", err))
				result = errors.Join(result, err)
			}
		}
		cancel()
		if waiter, ok := service.(Waiter); ok {
			waiter.Wait()
		}
	}
	return result
}

func closeResources(logger *slog.Logger, closers []Closer) error {
	var result error
	for i := len(closers) - 1; i >= 0; i-- {
		if closers[i] == nil {
			continue
		}
		if err := closers[i].Close(); err != nil {
			logger.Error("close resource", slog.Any("error", err))
			result = errors.Join(result, err)
		}
	}
	return result
}
