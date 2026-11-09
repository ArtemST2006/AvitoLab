package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthRepository implements Health interface
type HealthRepository struct {
	db *pgxpool.Pool
}

// NewHealthRepository returns a new obj of HealthRepository
func NewHealthRepository(db *pgxpool.Pool) *HealthRepository {
	return &HealthRepository{
		db: db,
	}
}

// Ping checks db available
func (h *HealthRepository) Ping(ctx context.Context) error {
	return h.db.Ping(ctx)
}
