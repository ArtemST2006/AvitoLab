// Package trip persists trips, status history, and idempotency keys.
package trip

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArtemST2006/AvitoLab/internal/infra/postgres/transaction"
)

// Repository persists trips and related records in PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

// New constructs the package component.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) db(ctx context.Context) executor {
	return transaction.Executor(ctx, r.pool)
}
