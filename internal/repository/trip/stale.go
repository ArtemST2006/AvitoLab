package trip

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"

	"github.com/ArtemST2006/AvitoLab/internal/model"
)

func (r *Repository) ListStale(ctx context.Context, staleAfter time.Duration) ([]model.StaleTrip, error) {
	query, args, err := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("id", "driver_id").
		From("trips").
		Where("status = 'active'").
		Where(squirrel.Expr(
			"COALESCE(last_position_at, started_at) < now() - (?::bigint * interval '1 microsecond')",
			staleAfter.Microseconds(),
		)).
		OrderBy("COALESCE(last_position_at, started_at) ASC", "id ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list stale trips: %w", err)
	}

	rows, err := r.db(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list stale trips: %w", err)
	}
	defer rows.Close()

	trips := make([]model.StaleTrip, 0)
	for rows.Next() {
		var item model.StaleTrip
		if err := rows.Scan(&item.TripID, &item.DriverID); err != nil {
			return nil, fmt.Errorf("scan stale trip: %w", err)
		}
		trips = append(trips, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stale trips: %w", err)
	}

	return trips, nil
}
