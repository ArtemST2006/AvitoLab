// Package main starts and stops the Trip Service.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/ArtemST2006/AvitoLab/internal/config"
	"github.com/ArtemST2006/AvitoLab/internal/container"
)

func main() {
	if err := run(); err != nil {
		slog.New(slog.NewJSONHandler(os.Stdout, nil)).With(
			slog.String("service", "trip-service"), slog.String("trace_id", ""), slog.String("span_id", ""),
		).
			Error("trip service failed", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := container.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize trip service: %w", err)
	}
	app.Log.Info("starting trip service", slog.String("address", app.Server.Addr))

	serverErr := make(chan error, 1)
	go func() { serverErr <- app.Server.ListenAndServe() }()
	var serveErr error
	select {
	case <-ctx.Done():
		app.Log.Info("shutdown signal received")
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			serveErr = err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	var shutdownErr error
	if err := app.Server.Shutdown(shutdownCtx); err != nil {
		_ = app.Server.Close()
		shutdownErr = fmt.Errorf("shut down HTTP server: %w", err)
	}
	if err := app.Close(shutdownCtx); err != nil {
		app.Log.Error("resource shutdown failed", slog.Any("error", err))
	}
	return errors.Join(serveErr, shutdownErr)
}
