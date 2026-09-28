// Package storage gives interfaces for work with PostgresQL
package storage

import (
	"context"
	"time"

	"github.com/google/uuid"

	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/ArtemST2006/AvitoLab/internal/schemas"
	"github.com/ArtemST2006/AvitoLab/internal/storage/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Trip — хранилище поездок.
type Trip interface {
	CreateTrip(ctx context.Context, data api.TripData) (api.Trip, error)
	GetTrip(ctx context.Context, tripID uuid.UUID) (api.Trip, error)
	FinishTrip(ctx context.Context, tripID api.TripId) (api.Trip, error)

	GetIdempotency(ctx context.Context, idempotencyKey string) (schemas.IdempotencyRecord, error)
	SetIdempotency(ctx context.Context, record schemas.IdempotencyRecord) error
}

// History — журнал смены статусов поездок.
type History interface {
	AppendRecord(ctx context.Context, tripID uuid.UUID, from *api.TripStatus, to api.TripStatus, at time.Time) error
}

// Health — проверка доступности хранилища.
type Health interface {
	Ping(context.Context) error
}

// Repository объединяет все хранилища сервиса.
type Repository struct {
	Trip
	Health
	History
}

// NewRepository создает Repository поверх пула PostgreSQL.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		Trip:    postgres.NewTripRepository(db),
		Health:  postgres.NewHealthRepository(db),
		History: postgres.NewHistoryRepository(db),
	}
}
