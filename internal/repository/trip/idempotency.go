// Package trip persists trips, status history, and idempotency keys.
package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domain "github.com/ArtemST2006/AvitoLab/internal/domain/trip"
	"github.com/ArtemST2006/AvitoLab/internal/model"
)

const idempotencyKeyColumn = "key"

// FindIdempotency loads a live idempotency key.
func (r *Repository) FindIdempotency(ctx context.Context, key uuid.UUID, expiresBefore time.Time) (model.IdempotencyRecord, error) {
	query, args, err := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select(idempotencyKeyColumn, "body_hash", "trip_id", "created_at").
		From("idempotency").Where(squirrel.Eq{idempotencyKeyColumn: key}).
		Where(squirrel.Gt{"created_at": expiresBefore}).ToSql()
	if err != nil {
		return model.IdempotencyRecord{}, fmt.Errorf("build select idempotency: %w", err)
	}
	var record model.IdempotencyRecord
	err = r.db(ctx).QueryRow(ctx, query, args...).Scan(
		&record.Key, &record.BodyHash, &record.TripID, &record.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.IdempotencyRecord{}, domain.ErrIdempotencyNotFound
	}
	if err != nil {
		return model.IdempotencyRecord{}, fmt.Errorf("select idempotency: %w", err)
	}
	return record, nil
}

// ReserveIdempotency claims a key if it is available.
func (r *Repository) ReserveIdempotency(ctx context.Context, key uuid.UUID, bodyHash string, expiresBefore time.Time) (bool, error) {
	builder := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	query, args, err := builder.Delete("idempotency").Where(squirrel.Eq{idempotencyKeyColumn: key}).
		Where(squirrel.LtOrEq{"created_at": expiresBefore}).ToSql()
	if err != nil {
		return false, fmt.Errorf("build delete expired idempotency: %w", err)
	}
	if _, deleteErr := r.db(ctx).Exec(ctx, query, args...); deleteErr != nil {
		return false, fmt.Errorf("delete expired idempotency: %w", deleteErr)
	}
	query, args, err = builder.Insert("idempotency").
		Columns(idempotencyKeyColumn, "body_hash").Values(key, bodyHash).
		Suffix("ON CONFLICT (key) DO NOTHING").ToSql()
	if err != nil {
		return false, fmt.Errorf("build reserve idempotency: %w", err)
	}
	tag, err := r.db(ctx).Exec(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("reserve idempotency: %w", err)
	}
	return tag.RowsAffected() != 0, nil
}

// LinkIdempotency links a reserved key to its trip.
func (r *Repository) LinkIdempotency(ctx context.Context, key, tripID uuid.UUID) error {
	query, args, err := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Update("idempotency").Set("trip_id", tripID).
		Where(squirrel.Eq{idempotencyKeyColumn: key}).ToSql()
	if err != nil {
		return fmt.Errorf("build link idempotency: %w", err)
	}
	tag, err := r.db(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("link idempotency: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrIdempotencyNotFound
	}
	return nil
}
