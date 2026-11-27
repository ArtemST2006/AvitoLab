package trip_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ArtemST2006/AvitoLab/internal/domain/trip"
)

func TestTripFinish(t *testing.T) {
	startedAt := time.Date(2026, time.November, 9, 10, 0, 0, 0, time.UTC)
	initialFinish := startedAt.Add(time.Hour)
	finishAt := startedAt.Add(2 * time.Hour)

	tests := []struct {
		name             string
		at               time.Time
		alreadyCompleted bool
		wantErr          error
	}{
		{name: "active trip", at: finishAt},
		{name: "active trip normalizes time to UTC", at: finishAt.In(time.FixedZone("MSK", 3*60*60))},
		{name: "completed trip stays unchanged", at: finishAt, alreadyCompleted: true, wantErr: trip.ErrTripCompleted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			journey, err := trip.New(
				uuid.New(), uuid.New(), uuid.New(),
				trip.Coordinates{Latitude: 55.75, Longitude: 37.62},
				trip.Coordinates{Latitude: 59.93, Longitude: 30.32},
				100, startedAt,
			)
			require.NoError(t, err)
			if tt.alreadyCompleted {
				require.NoError(t, journey.Finish(initialFinish))
			}
			before := journey.Snapshot()

			err = journey.Finish(tt.at)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, before, journey.Snapshot(), "failed Finish must not change trip state")
				return
			}
			require.NoError(t, err)
			want := before
			want.Status = trip.StatusCompleted
			finishedAt := tt.at.UTC()
			want.FinishedAt = &finishedAt
			state := journey.Snapshot()
			assert.Equal(t, want, state)
			require.NotNil(t, state.FinishedAt)
			assert.Equal(t, time.UTC, state.FinishedAt.Location())
		})
	}
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
	assert.Equal(t, trip.StatusActive, first.Status)
	assert.EqualValues(t, -1, first.Price)
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
