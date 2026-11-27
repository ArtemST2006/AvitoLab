package position

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"github.com/ArtemST2006/AvitoLab/internal/model"
)

// ListByTripID returns all trip positions in route order.
func (r *Repository) ListByTripID(ctx context.Context, tripID uuid.UUID) ([]model.Position, error) {
	query, args, err := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("id", "trip_id", "latitude", "longitude", "recorded_at").
		From("trip_positions").
		Where(squirrel.Eq{"trip_id": tripID}).
		OrderBy("recorded_at ASC", "id ASC").ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list trip positions: %w", err)
	}

	rows, err := r.db(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list trip positions: %w", err)
	}
	defer rows.Close()

	positions := make([]model.Position, 0)
	for rows.Next() {
		var position model.Position
		if err := rows.Scan(&position.ID, &position.TripID,
			&position.Coordinates.Latitude, &position.Coordinates.Longitude, &position.RecordedAt); err != nil {
			return nil, fmt.Errorf("scan trip position: %w", err)
		}
		positions = append(positions, position)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trip positions: %w", err)
	}
	return positions, nil
}
