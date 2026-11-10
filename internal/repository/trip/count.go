package trip

import (
	"context"
	"fmt"
)

// CountActive reports the current number of active trips.
func (r *Repository) CountActive(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db(ctx).QueryRow(ctx, "SELECT COUNT(*) FROM trips WHERE status = 'active'").Scan(&count); err != nil {
		return 0, fmt.Errorf("count active trips: %w", err)
	}
	return count, nil
}
