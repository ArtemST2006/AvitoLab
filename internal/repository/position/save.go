package position

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"

	"github.com/ArtemST2006/AvitoLab/internal/model"
)

// Insert stores a coordinate and returns its generated ID.
func (r *Repository) Insert(ctx context.Context, input model.SavePositionInput) (model.Position, error) {
	query, args, err := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Insert("trip_positions").
		Columns("trip_id", "latitude", "longitude", "recorded_at").
		Values(input.TripID, input.Coordinates.Latitude, input.Coordinates.Longitude, input.RecordedAt).
		Suffix("RETURNING id").ToSql()
	if err != nil {
		return model.Position{}, fmt.Errorf("build insert position: %w", err)
	}

	position := model.Position{
		TripID:      input.TripID,
		Coordinates: input.Coordinates,
		RecordedAt:  input.RecordedAt,
	}
	if err := r.db(ctx).QueryRow(ctx, query, args...).Scan(&position.ID); err != nil {
		return model.Position{}, fmt.Errorf("insert position: %w", err)
	}
	return position, nil
}
