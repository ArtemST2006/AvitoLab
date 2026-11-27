//go:build integration

package integration_test

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
	"github.com/ArtemST2006/AvitoLab/internal/model"
	positionrepo "github.com/ArtemST2006/AvitoLab/internal/repository/position"
	triprepo "github.com/ArtemST2006/AvitoLab/internal/repository/trip"
	save "github.com/ArtemST2006/AvitoLab/internal/usecase/position/save"
)

type failAfterPositionInsert struct {
	*triprepo.Repository
	err error
}

func (r failAfterPositionInsert) AdvanceLastPositionAt(context.Context, uuid.UUID, time.Time) error {
	return r.err
}

func TestSavePositionUpdatesLatestTimeAndRollsBackOnFailure(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	require.NotEmpty(t, dsn, "set DATABASE_URL and run make migrate before integration tests")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(ctx))

	trips := triprepo.New(pool)
	positions := positionrepo.New(pool)
	tx := transaction.New(pool)
	tripID := uuid.New()
	entity, err := domain.New(tripID, uuid.New(), uuid.New(),
		domain.Coordinates{Latitude: 55.75, Longitude: 37.62},
		domain.Coordinates{Latitude: 59.93, Longitude: 30.31},
		100, time.Now().UTC().Add(-5*time.Minute))
	require.NoError(t, err)
	require.NoError(t, trips.Insert(ctx, entity))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, cleanupErr := pool.Exec(cleanupCtx, "DELETE FROM trips WHERE id = $1", tripID)
		if cleanupErr != nil {
			t.Errorf("clean up trip %s: %v", tripID, cleanupErr)
		}
	})

	usecase := save.New(positions, trips, tx)
	latest := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	input := model.SavePositionInput{
		TripID: tripID, Coordinates: model.Coordinates{Latitude: 59.935, Longitude: 30.335},
		RecordedAt: latest,
	}
	first, err := usecase.Execute(ctx, input)
	require.NoError(t, err)
	require.Positive(t, first.ID)

	input.RecordedAt = latest.Add(-time.Minute)
	second, err := usecase.Execute(ctx, input)
	require.NoError(t, err)
	require.Positive(t, second.ID)
	input.RecordedAt = latest
	third, err := usecase.Execute(ctx, input)
	require.NoError(t, err)
	require.Positive(t, third.ID)

	route, err := positions.ListByTripID(ctx, tripID)
	require.NoError(t, err)
	require.Len(t, route, 3)
	require.Equal(t, []int64{second.ID, first.ID, third.ID},
		[]int64{route[0].ID, route[1].ID, route[2].ID},
		"route must be ordered by recorded_at and then id")

	var lastPositionAt time.Time
	var count int
	err = pool.QueryRow(ctx, "SELECT last_position_at FROM trips WHERE id = $1", tripID).Scan(&lastPositionAt)
	require.NoError(t, err)
	require.True(t, latest.Equal(lastPositionAt), "an older position must not move last_position_at backwards")
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM trip_positions WHERE trip_id = $1", tripID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 3, count)

	forcedError := errors.New("fail after position insert")
	failing := save.New(positions, failAfterPositionInsert{Repository: trips, err: forcedError}, tx)
	input.RecordedAt = latest.Add(5 * time.Second)
	_, err = failing.Execute(ctx, input)
	require.ErrorIs(t, err, forcedError)

	err = pool.QueryRow(ctx, "SELECT last_position_at FROM trips WHERE id = $1", tripID).Scan(&lastPositionAt)
	require.NoError(t, err)
	require.True(t, latest.Equal(lastPositionAt))
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM trip_positions WHERE trip_id = $1", tripID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 3, count, "failed transaction must roll back its inserted position")
}
