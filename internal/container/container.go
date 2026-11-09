// Package container assembles the service dependencies.
package container

import (
	"context"
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
	close  func()
}

// Close releases the application's resources.
func (a *App) Close() {
	a.close()
}

// New is the composition root for concrete dependencies.
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	log := telemetry.NewLogger(cfg.LogLevel, cfg.Env)
	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("initialize PostgreSQL: %w", err)
	}

	repo := triprepo.New(pool)
	tx := transaction.New(pool)
	createUC := usecreate.New(repo, repo, tx)
	getUC := useget.New(repo)
	finishUC := usefinish.New(repo, tx)

	handler, err := router.New(log, &router.Server{
		Create:      httpcreate.New(createUC, log, cfg.Database.QueryTimeout),
		Get:         httpget.New(getUC, log, cfg.Database.QueryTimeout),
		Finish:      httpfinish.New(finishUC, log, cfg.Database.QueryTimeout),
		HealthCheck: health.New(),
		ReadyCheck:  ready.New(pool, log, cfg.Database.QueryTimeout),
	})
	if err != nil {
		pool.Close()
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
		Log:   log,
		close: pool.Close,
	}, nil
}
