// Package trip persists trips, status history, and idempotency keys.
package trip

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domain "github.com/ArtemST2006/AvitoLab/internal/domain/trip"
)

// Get loads a trip by ID.
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*domain.Trip, error) {
	return r.fetch(ctx, id, false)
}

// GetForUpdate loads and locks a trip within a transaction.
func (r *Repository) GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.Trip, error) {
	return r.fetch(ctx, id, true)
}

func (r *Repository) fetch(ctx context.Context, id uuid.UUID, forUpdate bool) (*domain.Trip, error) {
	builder := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("id", "user_id", "driver_id", "start_latitude", "start_longitude",
			"end_latitude", "end_longitude", "price", "status", "started_at", "finished_at").
		From("trips").Where(squirrel.Eq{"id": id})
	if forUpdate {
		builder = builder.Suffix("FOR UPDATE")
	}
	query, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select trip: %w", err)
	}
	var state domain.State
	err = r.db(ctx).QueryRow(ctx, query, args...).Scan(
		&state.ID, &state.UserID, &state.DriverID,
		&state.StartPoint.Latitude, &state.StartPoint.Longitude,
		&state.EndPoint.Latitude, &state.EndPoint.Longitude,
		&state.Price, &state.Status, &state.StartedAt, &state.FinishedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrTripNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select trip: %w", err)
	}
	entity, err := domain.Restore(state)
	if err != nil {
		return nil, fmt.Errorf("restore trip: %w", err)
	}
	return entity, nil
}
