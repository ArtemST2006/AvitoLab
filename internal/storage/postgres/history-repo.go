package postgres

import (
	"context"
	"fmt"
	"time"

	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HistoryRepository хранит историю смены статусов поездок.
type HistoryRepository struct {
	pool *pgxpool.Pool
}

// NewHistoryRepository создает HistoryRepository.
func NewHistoryRepository(db *pgxpool.Pool) *HistoryRepository {
	return &HistoryRepository{pool: db}
}

// AppendRecord добавляет запись в историю. from == nil — поездка только создана, from_status будет NULL.
func (h *HistoryRepository) AppendRecord(ctx context.Context, tripID uuid.UUID, from *api.TripStatus, to api.TripStatus, at time.Time) error {
	query, args, err := squirrel.StatementBuilder.
		PlaceholderFormat(squirrel.Dollar).
		Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "changed_at").
		Values(tripID, from, to, at).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert trip status history: %w", err)
	}

	if _, err := giver(ctx, h.pool).Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("insert trip status history: %w", err)
	}

	return nil
}
