// Package trip persists trips, status history, and idempotency keys.
package trip

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgconn"

	domain "github.com/ArtemST2006/AvitoLab/internal/domain/trip"
)

// Insert stores a new trip.
func (r *Repository) Insert(ctx context.Context, entity *domain.Trip) error {
	state := entity.Snapshot()
	query, args, err := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Insert("trips").
		Columns("id", "user_id", "driver_id", "start_latitude", "start_longitude",
			"end_latitude", "end_longitude", "price", "status", "started_at").
		Values(state.ID, state.UserID, state.DriverID, state.StartPoint.Latitude,
			state.StartPoint.Longitude, state.EndPoint.Latitude, state.EndPoint.Longitude,
			state.Price, state.Status, state.StartedAt).ToSql()
	if err != nil {
		return fmt.Errorf("build insert trip: %w", err)
	}
	_, err = r.db(ctx).Exec(ctx, query, args...)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "trips_driver_active_uidx" {
		return domain.ErrDriverBusy
	}
	if err != nil {
		return fmt.Errorf("insert trip: %w", err)
	}
	return nil
}

// Update persists a trip state transition.
func (r *Repository) Update(ctx context.Context, entity *domain.Trip) error {
	state := entity.Snapshot()
	query, args, err := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Update("trips").
		Set("status", state.Status).
		Set("finished_at", state.FinishedAt).
		Set("updated_at", state.FinishedAt).
		Where(squirrel.Eq{"id": state.ID}).ToSql()
	if err != nil {
		return fmt.Errorf("build update trip: %w", err)
	}
	tag, err := r.db(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update trip: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTripNotFound
	}
	return nil
}
