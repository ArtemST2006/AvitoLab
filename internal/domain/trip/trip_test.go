package trip_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ArtemST2006/AvitoLab/internal/domain/trip"
)

func TestTripFinishActive(t *testing.T) {
	finishedAt := time.Date(2026, time.November, 9, 12, 0, 0, 0, time.UTC)
	journey, err := trip.New(
		uuid.New(), uuid.New(), uuid.New(),
		trip.Coordinates{Latitude: 66.3, Longitude: 23.4},
		trip.Coordinates{Latitude: 34.2, Longitude: 34.4},
		100, finishedAt.Add(-time.Minute),
	)

	require.NoError(t, err)

	require.NoError(t, journey.Finish(finishedAt))

	state := journey.Snapshot()

	assert.Equal(t, trip.StatusCompleted, state.Status)
	require.NotNil(t, state.FinishedAt)
	assert.Equal(t, finishedAt, *state.FinishedAt)
}

func TestTripFinishCompleted(t *testing.T) {
	finishedAt := time.Date(2026, time.November, 9, 12, 0, 0, 0, time.UTC)
	journey, err := trip.New(
		uuid.New(), uuid.New(), uuid.New(),
		trip.Coordinates{Latitude: 55.75, Longitude: 37.62},
		trip.Coordinates{Latitude: 59.93, Longitude: 30.32},
		100, finishedAt.Add(-time.Minute),
	)
	require.NoError(t, err)
	require.NoError(t, journey.Finish(finishedAt))
	before := journey.Snapshot()

	err = journey.Finish(finishedAt.Add(time.Hour))

	require.ErrorIs(t, err, trip.ErrTripCompleted)
	assert.Equal(t, before, journey.Snapshot(), "a repeated Finish must not change trip state")
}

func TestTripFinishNormalizesTimeToUTC(t *testing.T) {
	localTime := time.Date(2026, time.November, 9, 15, 0, 0, 0, time.FixedZone("MSK", 3*60*60))
	journey, err := trip.New(
		uuid.New(), uuid.New(), uuid.New(),
		trip.Coordinates{}, trip.Coordinates{}, 0, localTime.Add(-time.Minute),
	)
	require.NoError(t, err)

	require.NoError(t, journey.Finish(localTime))

	state := journey.Snapshot()
	require.NotNil(t, state.FinishedAt)
	assert.Equal(t, localTime.UTC(), *state.FinishedAt)
	assert.Equal(t, time.UTC, state.FinishedAt.Location())
}

func TestTripSnapshotDoesNotExposeMutableState(t *testing.T) {
	finishedAt := time.Date(2026, time.November, 9, 12, 0, 0, 0, time.UTC)
	lastPositionAt := finishedAt.Add(-time.Minute)
	wantFinishedAt := finishedAt
	wantLastPositionAt := lastPositionAt
	state := validState()
	state.Status = trip.StatusCompleted
	state.FinishedAt = &finishedAt
	state.LastPositionAt = &lastPositionAt
	journey, err := trip.Restore(state)
	require.NoError(t, err)

	// The caller still owns the pointers passed to Restore.
	finishedAt = finishedAt.Add(time.Hour)
	lastPositionAt = lastPositionAt.Add(time.Hour)
	first := journey.Snapshot()
	require.NotNil(t, first.FinishedAt)
	require.NotNil(t, first.LastPositionAt)
	assert.Equal(t, wantFinishedAt, *first.FinishedAt)
	assert.Equal(t, wantLastPositionAt, *first.LastPositionAt)

	// A snapshot must also be safe to change after it is returned.
	first.Status = trip.StatusActive
	first.Price = -1
	*first.FinishedAt = first.FinishedAt.Add(time.Hour)
	*first.LastPositionAt = first.LastPositionAt.Add(time.Hour)
	second := journey.Snapshot()
	assert.Equal(t, trip.StatusCompleted, second.Status)
	assert.Equal(t, state.Price, second.Price)
	require.NotNil(t, second.FinishedAt)
	require.NotNil(t, second.LastPositionAt)
	assert.Equal(t, wantFinishedAt, *second.FinishedAt)
	assert.Equal(t, wantLastPositionAt, *second.LastPositionAt)
}
