// Package postgres отвечает за работу с PostgreSQL.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArtemST2006/AvitoLab/internal/config"
)

// DBTX реализуют и *pgxpool.Pool, и pgx.Tx. Репозиторий получает его через giver:
// внутри TransactionManager.Do это транзакция из контекста, вне — пул.
type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

var (
	// ErrDriverBusy — у водителя уже есть активная поездка.
	ErrDriverBusy = errors.New("driver busy")

	// ErrNotFound — запрошенной строки в PostgreSQL нет.
	ErrNotFound = errors.New("not found")

	// ErrIdempotencyExists — живой ключ идемпотентности уже занят другим запросом.
	ErrIdempotencyExists = errors.New("idempotency key exists")
)

// NewPool создает пул подключений к PostgreSQL и проверяет, что база доступна.
func NewPool(ctx context.Context, cfg config.Database) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
