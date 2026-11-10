// Package container assembles the service dependencies.
package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/ArtemST2006/AvitoLab/internal/config"
	"github.com/ArtemST2006/AvitoLab/internal/http/health"
	"github.com/ArtemST2006/AvitoLab/internal/http/ready"
	"github.com/ArtemST2006/AvitoLab/internal/http/router"
	httpcreate "github.com/ArtemST2006/AvitoLab/internal/http/trip/create"
	httpfinish "github.com/ArtemST2006/AvitoLab/internal/http/trip/finish"
	httpget "github.com/ArtemST2006/AvitoLab/internal/http/trip/get"
	"github.com/ArtemST2006/AvitoLab/internal/infra/postgres"
	"github.com/ArtemST2006/AvitoLab/internal/infra/postgres/transaction"
	"github.com/ArtemST2006/AvitoLab/internal/infra/telemetry"
	triprepo "github.com/ArtemST2006/AvitoLab/internal/repository/trip"
	usecreate "github.com/ArtemST2006/AvitoLab/internal/usecase/trip/create"
	usefinish "github.com/ArtemST2006/AvitoLab/internal/usecase/trip/finish"
	useget "github.com/ArtemST2006/AvitoLab/internal/usecase/trip/get"
)

// App owns the HTTP server and cleanup function.
type App struct {
	Server *http.Server
	Log    *slog.Logger
	close  func(context.Context) error
}

// Close releases the application's resources.
func (a *App) Close(ctx context.Context) error {
	return a.close(ctx)
}

// New is the composition root for concrete dependencies.
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	log, shutdown, err := telemetry.Setup(ctx, "trip-service", cfg.LogLevel, cfg.Env)
	if err != nil {
		return nil, fmt.Errorf("initialize OpenTelemetry: %w", err)
	}
	cleanupTelemetry := func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
		defer cancel()
		if err := shutdown(shutdownCtx); err != nil {
			log.Error("OpenTelemetry shutdown failed", slog.Any("error", err))
		}
	}
	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		cleanupTelemetry()
		return nil, fmt.Errorf("initialize PostgreSQL: %w", err)
	}

	repo := triprepo.New(pool)
	tx := transaction.New(pool)
	tripMetrics, err := telemetry.NewTripMetrics()
	if err != nil {
		pool.Close()
		cleanupTelemetry()
		return nil, fmt.Errorf("initialize trip metrics: %w", err)
	}
	activeTrips, err := telemetry.RegisterActiveTrips(repo, cfg.Database.QueryTimeout)
	if err != nil {
		pool.Close()
		cleanupTelemetry()
		return nil, fmt.Errorf("initialize active trips gauge: %w", err)
	}
	createUC := usecreate.New(repo, repo, tx, tripMetrics)
	getUC := useget.New(repo)
	finishUC := usefinish.New(repo, tx, tripMetrics)

	handler, err := router.New(log, &router.Server{
		Create:      httpcreate.New(createUC, log, cfg.Database.QueryTimeout),
		Get:         httpget.New(getUC, log, cfg.Database.QueryTimeout),
		Finish:      httpfinish.New(finishUC, log, cfg.Database.QueryTimeout),
		HealthCheck: health.New(),
		ReadyCheck:  ready.New(pool, log, cfg.Database.QueryTimeout),
	})
	if err != nil {
		_ = activeTrips.Unregister()
		pool.Close()
		cleanupTelemetry()
		return nil, fmt.Errorf("initialize HTTP router: %w", err)
	}
	return &App{
		Server: &http.Server{
			Addr: cfg.HTTPServer.Address, Handler: handler,
			ReadHeaderTimeout: cfg.HTTPServer.ReadHeaderTimeout,
			ReadTimeout:       cfg.HTTPServer.ReadTimeout,
			WriteTimeout:      cfg.HTTPServer.WriteTimeout,
			IdleTimeout:       cfg.HTTPServer.IdleTimeout,
		},
		Log: log,
		close: func(ctx context.Context) error {
			unregisterErr := activeTrips.Unregister()
			telemetryErr := shutdown(ctx)
			poolClosed := make(chan struct{})
			go func() {
				pool.Close()
				close(poolClosed)
			}()
			select {
			case <-poolClosed:
				return errors.Join(unregisterErr, telemetryErr)
			case <-ctx.Done():
				return errors.Join(unregisterErr, telemetryErr, fmt.Errorf("close PostgreSQL pool: %w", ctx.Err()))
			}
		},
	}, nil
}
