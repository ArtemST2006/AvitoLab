// Package position persists trip coordinates in PostgreSQL.
package position

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArtemST2006/AvitoLab/internal/infra/postgres/transaction"
)

// Repository persists positions.
type Repository struct {
	pool *pgxpool.Pool
}

// New constructs the package component.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) db(ctx context.Context) transaction.DBTX {
	return transaction.Executor(ctx, r.pool)
}
