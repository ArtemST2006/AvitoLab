// Package trip persists trips, status history, and idempotency keys.
package trip

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	domain "github.com/ArtemST2006/AvitoLab/internal/domain/trip"
)

// AppendStatus records a trip state change.
func (r *Repository) AppendStatus(ctx context.Context, id uuid.UUID, from *domain.Status, to domain.Status, at time.Time) error {
	query, args, err := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "changed_at").
		Values(id, from, to, at).ToSql()
	if err != nil {
		return fmt.Errorf("build insert trip status history: %w", err)
	}
	if _, err := r.db(ctx).Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("insert trip status history: %w", err)
	}
	return nil
}
