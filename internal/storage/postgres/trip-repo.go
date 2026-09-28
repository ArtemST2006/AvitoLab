package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// uniqueViolationCode — SQLSTATE нарушения уникального индекса.
const uniqueViolationCode = "23505"

// TripRepository хранит поездки в PostgreSQL.
type TripRepository struct {
	pool *pgxpool.Pool
}

// NewTripRepository создает TripRepository.
func NewTripRepository(db *pgxpool.Pool) *TripRepository {
	return &TripRepository{pool: db}
}

// CreateTrip сохраняет новую активную поездку. ErrDriverBusy - если у водителя уже есть активная поездка.
func (r *TripRepository) CreateTrip(ctx context.Context, data api.TripData) (api.Trip, error) {
	t := api.Trip{
		Id:         uuid.New(),
		UserId:     data.UserId,
		DriverId:   data.DriverId,
		StartPoint: data.StartPoint,
		EndPoint:   data.EndPoint,
		Price:      data.Price,
		Status:     api.Active,
		StartedAt:  time.Now().UTC(),
	}

	query, args, err := squirrel.StatementBuilder.
		PlaceholderFormat(squirrel.Dollar).
		Insert("trips").
		Columns(
			"id", "user_id", "driver_id",
			"start_latitude", "start_longitude", "end_latitude", "end_longitude",
			"price", "status", "started_at",
		).
		Values(
			t.Id, t.UserId, t.DriverId,
			t.StartPoint.Latitude, t.StartPoint.Longitude, t.EndPoint.Latitude, t.EndPoint.Longitude,
			t.Price, t.Status, t.StartedAt,
		).
		ToSql()
	if err != nil {
		return api.Trip{}, fmt.Errorf("build insert trip: %w", err)
	}

	_, err = giver(ctx, r.pool).Exec(ctx, query, args...)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
		return api.Trip{}, ErrDriverBusy
	}
	if err != nil {
		return api.Trip{}, fmt.Errorf("insert trip: %w", err)
	}

	return t, nil
}

// GetTrip возвращает поездку по id и блокирует строку.
func (r *TripRepository) GetTrip(ctx context.Context, tripID uuid.UUID) (api.Trip, error) {
	query, args, err := squirrel.StatementBuilder.
		PlaceholderFormat(squirrel.Dollar).
		Select(
			"id", "user_id", "driver_id",
			"start_latitude", "start_longitude", "end_latitude", "end_longitude",
			"price", "status", "started_at", "finished_at",
		).
		From("trips").
		Where(squirrel.Eq{"id": tripID}).
		Suffix("FOR UPDATE").
		ToSql()
	if err != nil {
		return api.Trip{}, fmt.Errorf("build select trip: %w", err)
	}

	var t api.Trip
	err = giver(ctx, r.pool).QueryRow(ctx, query, args...).Scan(
		&t.Id, &t.UserId, &t.DriverId,
		&t.StartPoint.Latitude, &t.StartPoint.Longitude, &t.EndPoint.Latitude, &t.EndPoint.Longitude,
		&t.Price, &t.Status, &t.StartedAt, &t.FinishedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return api.Trip{}, ErrNotFound
	}
	if err != nil {
		return api.Trip{}, fmt.Errorf("select trip: %w", err)
	}

	return t, nil
}

// FinishTrip переводит поездку в completed и возвращает ее. Если поездки нет — ErrNotFound.
func (r *TripRepository) FinishTrip(ctx context.Context, tripID api.TripId) (api.Trip, error) {
	finishedAt := time.Now().UTC()

	query, args, err := squirrel.StatementBuilder.
		PlaceholderFormat(squirrel.Dollar).
		Update("trips").
		Set("status", api.Completed).
		Set("finished_at", finishedAt).
		Set("updated_at", finishedAt).
		Where(squirrel.Eq{"id": tripID}).
		Suffix("RETURNING id, user_id, driver_id, start_latitude, start_longitude, end_latitude, end_longitude, price, status, started_at, finished_at").
		ToSql()
	if err != nil {
		return api.Trip{}, fmt.Errorf("build update trip: %w", err)
	}

	var t api.Trip
	err = giver(ctx, r.pool).QueryRow(ctx, query, args...).Scan(
		&t.Id, &t.UserId, &t.DriverId,
		&t.StartPoint.Latitude, &t.StartPoint.Longitude, &t.EndPoint.Latitude, &t.EndPoint.Longitude,
		&t.Price, &t.Status, &t.StartedAt, &t.FinishedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return api.Trip{}, ErrNotFound
	}
	if err != nil {
		return api.Trip{}, fmt.Errorf("update trip: %w", err)
	}

	return t, nil
}
