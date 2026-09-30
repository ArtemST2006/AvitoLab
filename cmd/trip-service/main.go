// Package main implements backend for avito lab
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ArtemST2006/AvitoLab/internal/config"
	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/ArtemST2006/AvitoLab/internal/http-server/apierr"
	"github.com/ArtemST2006/AvitoLab/internal/http-server/handlers"
	slogpretty "github.com/ArtemST2006/AvitoLab/internal/lib/logger/handlers"
	"github.com/ArtemST2006/AvitoLab/internal/lib/logger/sl"
	"github.com/ArtemST2006/AvitoLab/internal/storage"
	"github.com/ArtemST2006/AvitoLab/internal/storage/postgres"
	"github.com/go-chi/chi/v5"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	cfg := config.MustLoad()

	log := setupLogger(cfg.LogLevel, cfg.Env)

	log.Info("starting trip-service", slog.String("env", cfg.Env), slog.String("logLevel", cfg.LogLevel))
	log.Info("info messages are enabled")
	log.Debug("debug messages are enabled")
	log.Error("error messages are enabled")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		log.Error("failed to connect to PostgreSQL", sl.Err(err))
		return
	}
	defer pool.Close()

	log.Info("PostgreSQL up with pgxpool")

	router := chi.NewRouter()

	repo := storage.NewRepository(pool)
	txManager := postgres.NewTransactionManager(pool)
	api.HandlerWithOptions(handlers.NewServer(log, repo, txManager, cfg.Database.QueryTimeout), api.ChiServerOptions{
		BaseRouter: router,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			apierr.Write(w, r, apierr.InvalidRequest(err.Error()))
		},
	})

	srv := http.Server{
		Addr:              cfg.HTTPServer.Address,
		Handler:           router,
		ReadHeaderTimeout: cfg.HTTPServer.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTPServer.ReadTimeout,
		WriteTimeout:      cfg.HTTPServer.WriteTimeout,
		IdleTimeout:       cfg.HTTPServer.IdleTimeout,
	}

	log.Info("starting server", slog.String("address", cfg.HTTPServer.Address))

	serverErr := make(chan error, 1)
	go func() {
		log.Info("starting server", slog.String("address", cfg.HTTPServer.Address))
		if err := srv.ListenAndServe(); err != nil {
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-serverErr:
		log.Error("failed to start server", sl.Err(err))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	log.Info("server shutting down")

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown timed out, closing connections", sl.Err(err))
		_ = srv.Close()
	}

	log.Info("server stopped")
}

func setupLogger(logLevel, env string) *slog.Logger {
	var log *slog.Logger

	var slogLevel slog.Level

	switch strings.ToLower(logLevel) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	switch env {
	case envDev:
		log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel}))
	case envProd:
		log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel}))
	default:
		log = setupPrettySlog(slogLevel)
	}

	return log
}

func setupPrettySlog(logLevel slog.Level) *slog.Logger {
	opts := slogpretty.PrettyHandlerOptions{
		SlogOpts: &slog.HandlerOptions{
			Level: logLevel,
		},
	}

	handler := opts.NewPrettyHandler(os.Stdout)

	return slog.New(handler)
}
