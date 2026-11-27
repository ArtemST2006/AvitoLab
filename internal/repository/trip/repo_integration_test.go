//go:build integration

package trip_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	domain "github.com/ArtemST2006/AvitoLab/internal/domain/trip"
	"github.com/ArtemST2006/AvitoLab/internal/infra/postgres/transaction"
	repository "github.com/ArtemST2006/AvitoLab/internal/repository/trip"
)

type testDB struct {
	ctx   context.Context
	pool  *pgxpool.Pool
	repo  *repository.Repository
	tx    *transaction.Manager
	trips []uuid.UUID
	keys  []uuid.UUID
}

func openTestDB(t *testing.T) *testDB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	require.NotEmpty(t, dsn, "set DATABASE_URL and run make migrate before integration tests")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(ctx))

	db := &testDB{ctx: ctx, pool: pool, repo: repository.New(pool), tx: transaction.New(pool)}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		for _, key := range db.keys {
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM idempotency WHERE key = $1", key); err != nil {
				t.Errorf("clean up idempotency key %s: %v", key, err)
			}
		}
		for _, id := range db.trips {
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM trips WHERE id = $1", id); err != nil {
				t.Errorf("clean up trip %s: %v", id, err)
			}
		}
	})
	return db
}

func (db *testDB) newTrip(t *testing.T, driverID uuid.UUID) *domain.Trip {
	t.Helper()
	id := uuid.New()
	db.trips = append(db.trips, id)
	entity, err := domain.New(id, uuid.New(), driverID,
		domain.Coordinates{Latitude: 55.75, Longitude: 37.62},
		domain.Coordinates{Latitude: 59.93, Longitude: 30.31},
		100, time.Now().UTC().Truncate(time.Microsecond))
	require.NoError(t, err)
	return entity
}

func (db *testDB) newKey() uuid.UUID {
	key := uuid.New()
	db.keys = append(db.keys, key)
	return key
}

func TestRepositoryInsertGetAndHistory(t *testing.T) {
	db := openTestDB(t)
	entity := db.newTrip(t, uuid.New())
	state := entity.Snapshot()

	require.NoError(t, db.repo.Insert(db.ctx, entity))
	require.NoError(t, db.repo.AppendStatus(db.ctx, state.ID, nil, state.Status, state.StartedAt))

	got, err := db.repo.Get(db.ctx, state.ID)
	require.NoError(t, err)
	actual := got.Snapshot()
	require.Equal(t, state.ID, actual.ID)
	require.Equal(t, state.UserID, actual.UserID)
	require.Equal(t, state.DriverID, actual.DriverID)
	require.Equal(t, state.StartPoint, actual.StartPoint)
	require.Equal(t, state.EndPoint, actual.EndPoint)
	require.Equal(t, state.Price, actual.Price)
	require.Equal(t, domain.StatusActive, actual.Status)
	require.True(t, state.StartedAt.Equal(actual.StartedAt))
	require.Nil(t, actual.FinishedAt)

	var from *string
	var to string
	var changedAt time.Time
	err = db.pool.QueryRow(db.ctx,
		"SELECT from_status, to_status, changed_at FROM trip_status_history WHERE trip_id = $1", state.ID).
		Scan(&from, &to, &changedAt)
	require.NoError(t, err)
	require.Nil(t, from)
	require.Equal(t, string(domain.StatusActive), to)
	require.True(t, state.StartedAt.Equal(changedAt))
}

func TestRepositoryFinishAndGetForUpdate(t *testing.T) {
	db := openTestDB(t)
	entity := db.newTrip(t, uuid.New())
	id := entity.Snapshot().ID
	require.NoError(t, db.repo.Insert(db.ctx, entity))

	err := db.tx.Do(db.ctx, func(ctx context.Context) error {
		locked, err := db.repo.GetForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if err := locked.Finish(time.Now()); err != nil {
			return err
		}
		if err := db.repo.Update(ctx, locked); err != nil {
			return err
		}
		state := locked.Snapshot()
		from := domain.StatusActive
		return db.repo.AppendStatus(ctx, id, &from, state.Status, *state.FinishedAt)
	})
	require.NoError(t, err)

	got, err := db.repo.Get(db.ctx, id)
	require.NoError(t, err)
	state := got.Snapshot()
	require.Equal(t, domain.StatusCompleted, state.Status)
	require.NotNil(t, state.FinishedAt)
	var from, to string
	err = db.pool.QueryRow(db.ctx,
		"SELECT from_status, to_status FROM trip_status_history WHERE trip_id = $1", id).
		Scan(&from, &to)
	require.NoError(t, err)
	require.Equal(t, string(domain.StatusActive), from)
	require.Equal(t, string(domain.StatusCompleted), to)
}

func TestRepositoryDriverBusy(t *testing.T) {
	db := openTestDB(t)
	driverID := uuid.New()
	first := db.newTrip(t, driverID)
	second := db.newTrip(t, driverID)

	require.NoError(t, db.repo.Insert(db.ctx, first))
	require.ErrorIs(t, db.repo.Insert(db.ctx, second), domain.ErrDriverBusy)
	require.NoError(t, first.Finish(time.Now()))
	require.NoError(t, db.repo.Update(db.ctx, first))
	require.NoError(t, db.repo.Insert(db.ctx, second))
}

func TestRepositoryMissingTrip(t *testing.T) {
	db := openTestDB(t)
	id := uuid.New()
	_, err := db.repo.Get(db.ctx, id)
	require.ErrorIs(t, err, domain.ErrTripNotFound)

	entity := db.newTrip(t, uuid.New())
	require.NoError(t, entity.Finish(time.Now()))
	require.ErrorIs(t, db.repo.Update(db.ctx, entity), domain.ErrTripNotFound)
}

func TestRepositoryIdempotencyLifecycle(t *testing.T) {
	db := openTestDB(t)
	key := db.newKey()
	expiresBefore := time.Now().Add(-time.Hour)

	reserved, err := db.repo.ReserveIdempotency(db.ctx, key, "first-hash", expiresBefore)
	require.NoError(t, err)
	require.True(t, reserved)
	reserved, err = db.repo.ReserveIdempotency(db.ctx, key, "other-hash", expiresBefore)
	require.NoError(t, err)
	require.False(t, reserved)

	entity := db.newTrip(t, uuid.New())
	id := entity.Snapshot().ID
	require.NoError(t, db.repo.Insert(db.ctx, entity))
	require.NoError(t, db.repo.LinkIdempotency(db.ctx, key, id))
	record, err := db.repo.FindIdempotency(db.ctx, key, expiresBefore)
	require.NoError(t, err)
	require.Equal(t, key, record.Key)
	require.Equal(t, "first-hash", record.BodyHash)
	require.Equal(t, id, record.TripID)

	_, err = db.pool.Exec(db.ctx,
		"UPDATE idempotency SET created_at = $1 WHERE key = $2", time.Now().Add(-2*time.Hour), key)
	require.NoError(t, err)
	_, err = db.repo.FindIdempotency(db.ctx, key, expiresBefore)
	require.ErrorIs(t, err, domain.ErrIdempotencyNotFound)
	reserved, err = db.repo.ReserveIdempotency(db.ctx, key, "new-hash", expiresBefore)
	require.NoError(t, err)
	require.True(t, reserved)

	var hash string
	var linkedID *uuid.UUID
	err = db.pool.QueryRow(db.ctx, "SELECT body_hash, trip_id FROM idempotency WHERE key = $1", key).
		Scan(&hash, &linkedID)
	require.NoError(t, err)
	require.Equal(t, "new-hash", hash)
	require.Nil(t, linkedID)
	require.ErrorIs(t, db.repo.LinkIdempotency(db.ctx, uuid.New(), id), domain.ErrIdempotencyNotFound)
}

func TestRepositoryRollsBackTripAndHistory(t *testing.T) {
	db := openTestDB(t)
	entity := db.newTrip(t, uuid.New())
	state := entity.Snapshot()
	rollbackErr := errors.New("abort after both inserts")

	err := db.tx.Do(db.ctx, func(ctx context.Context) error {
		if err := db.repo.Insert(ctx, entity); err != nil {
			return err
		}
		if err := db.repo.AppendStatus(ctx, state.ID, nil, state.Status, state.StartedAt); err != nil {
			return err
		}
		return rollbackErr
	})
	require.ErrorIs(t, err, rollbackErr)

	var tripCount, historyCount int
	require.NoError(t, db.pool.QueryRow(db.ctx, "SELECT COUNT(*) FROM trips WHERE id = $1", state.ID).Scan(&tripCount))
	require.NoError(t, db.pool.QueryRow(db.ctx,
		"SELECT COUNT(*) FROM trip_status_history WHERE trip_id = $1", state.ID).Scan(&historyCount))
	require.Zero(t, tripCount)
	require.Zero(t, historyCount)
}
