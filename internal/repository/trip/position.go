package trip

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	domain "github.com/ArtemST2006/AvitoLab/internal/domain/trip"
)

// AdvanceLastPositionAt keeps the newest recorded coordinate time.
func (r *Repository) AdvanceLastPositionAt(ctx context.Context, id uuid.UUID, recordedAt time.Time) error {
	query, args, err := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Update("trips").
		Set("last_position_at", squirrel.Expr(
			"CASE WHEN last_position_at IS NULL OR last_position_at < ? THEN ? ELSE last_position_at END",
			recordedAt, recordedAt,
		)).
		Where(squirrel.Eq{"id": id}).ToSql()
	if err != nil {
		return fmt.Errorf("build update last position: %w", err)
	}
	tag, err := r.db(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update last position: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTripNotFound
	}
	return nil
}
