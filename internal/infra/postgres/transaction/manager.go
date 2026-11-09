// Package transaction manages PostgreSQL transactions.
package transaction

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type key struct{}

// DBTX is the common query surface of pgxpool.Pool and pgx.Tx.
type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Manager owns transaction creation for a PostgreSQL pool.
type Manager struct {
	pool *pgxpool.Pool
}

// New constructs the package component.
func New(pool *pgxpool.Pool) *Manager {
	return &Manager{pool: pool}
}

// Do uses Read Committed. Nested calls reuse the current transaction.
func (m *Manager) Do(ctx context.Context, fn func(context.Context) error) (err error) {
	if _, ok := ctx.Value(key{}).(pgx.Tx); ok {
		return fn(ctx)
	}
	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	txCtx := context.WithValue(ctx, key{}, tx)
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback(rollbackCtx)
			panic(recovered)
		}
		if err != nil {
			_ = tx.Rollback(rollbackCtx)
			return
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			err = fmt.Errorf("commit transaction: %w", commitErr)
		}
	}()
	return fn(txCtx)
}

// Executor returns the active transaction or the pool.
func Executor(ctx context.Context, pool *pgxpool.Pool) DBTX {
	if tx, ok := ctx.Value(key{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}
