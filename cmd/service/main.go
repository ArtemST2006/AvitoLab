// Package main starts and stops the Trip Service.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
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
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.MustLoad()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := container.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize trip service: %w", err)
	}
	defer app.Close()
	app.Log.Info("starting trip service", slog.String("address", app.Server.Addr))

	serverErr := make(chan error, 1)
	go func() { serverErr <- app.Server.ListenAndServe() }()
	var serveErr error
	select {
	case <-ctx.Done():
		app.Log.Info("shutdown signal received")
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			app.Log.Error("HTTP server stopped", slog.Any("error", err))
			serveErr = err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := app.Server.Shutdown(shutdownCtx); err != nil {
		app.Log.Error("graceful shutdown failed", slog.Any("error", err))
		_ = app.Server.Close()
		return fmt.Errorf("shut down HTTP server: %w", err)
	}
	return serveErr
}
