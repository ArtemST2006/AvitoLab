package finish_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ArtemST2006/AvitoLab/internal/domain/trip"
	"github.com/ArtemST2006/AvitoLab/internal/model"
	"github.com/ArtemST2006/AvitoLab/internal/usecase/trip/finish"
)

type fixture struct {
	ctx    context.Context
	id     uuid.UUID
	entity *trip.Trip
	trips  *MocktripRepository
	tx     *MocktxManager
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	id := uuid.New()
	entity, err := trip.New(id, uuid.New(), uuid.New(),
		trip.Coordinates{Latitude: 55.75, Longitude: 37.62},
		trip.Coordinates{Latitude: 59.93, Longitude: 30.31},
		100, time.Now().Add(-time.Minute))
	require.NoError(t, err)
	return &fixture{
		ctx: context.Background(), id: id, entity: entity,
		trips: NewMocktripRepository(ctrl), tx: NewMocktxManager(ctrl),
	}
}

func (f *fixture) expectTx(txErr error) {
	f.tx.EXPECT().Do(f.ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
		if txErr != nil {
			return txErr
		}
		return fn(ctx)
	})
}

func (f *fixture) execute() (model.Trip, error) {
	return finish.New(f.trips, f.tx).Execute(f.ctx, f.id)
}

func TestExecuteFinishesActiveTrip(t *testing.T) {
	f := newFixture(t)
	f.expectTx(nil)
	get := f.trips.EXPECT().GetForUpdate(f.ctx, f.id).Return(f.entity, nil)
	update := f.trips.EXPECT().Update(f.ctx, f.entity).After(get).
		DoAndReturn(func(_ context.Context, entity *trip.Trip) error {
			state := entity.Snapshot()
			require.Equal(t, trip.StatusCompleted, state.Status)
			require.NotNil(t, state.FinishedAt)
			return nil
		})
	f.trips.EXPECT().AppendStatus(f.ctx, f.id, gomock.Any(), trip.StatusCompleted, gomock.Any()).
		After(update).DoAndReturn(func(_ context.Context, _ uuid.UUID, from *trip.Status, to trip.Status, at time.Time) error {
		require.Equal(t, trip.StatusActive, *from)
		require.Equal(t, trip.StatusCompleted, to)
		require.Equal(t, *f.entity.Snapshot().FinishedAt, at)
		return nil
	})

	result, err := f.execute()

	require.NoError(t, err)
	require.Equal(t, model.FromDomain(f.entity), result)
	require.Equal(t, trip.StatusCompleted, result.Status)
	require.NotNil(t, result.FinishedAt)
}

func TestExecuteErrors(t *testing.T) {
	repoErr := errors.New("repository failed")
	cases := []struct {
		name   string
		want   error
		setup  func(*testing.T, *fixture)
		txFail bool
	}{
		{"trip not found", trip.ErrTripNotFound, func(_ *testing.T, f *fixture) {
			f.trips.EXPECT().GetForUpdate(f.ctx, f.id).Return(nil, trip.ErrTripNotFound)
		}, false},
		{"already completed", trip.ErrTripCompleted, func(t *testing.T, f *fixture) {
			require.NoError(t, f.entity.Finish(time.Now()))
			f.trips.EXPECT().GetForUpdate(f.ctx, f.id).Return(f.entity, nil)
		}, false},
		{"update failed", repoErr, func(_ *testing.T, f *fixture) {
			get := f.trips.EXPECT().GetForUpdate(f.ctx, f.id).Return(f.entity, nil)
			f.trips.EXPECT().Update(f.ctx, f.entity).After(get).Return(repoErr)
		}, false},
		{"history failed", repoErr, func(_ *testing.T, f *fixture) {
			get := f.trips.EXPECT().GetForUpdate(f.ctx, f.id).Return(f.entity, nil)
			update := f.trips.EXPECT().Update(f.ctx, f.entity).After(get).Return(nil)
			f.trips.EXPECT().AppendStatus(f.ctx, f.id, gomock.Any(), trip.StatusCompleted, gomock.Any()).
				After(update).Return(repoErr)
		}, false},
		{"transaction failed", repoErr, nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			if tc.txFail {
				f.expectTx(repoErr)
			} else {
				f.expectTx(nil)
				tc.setup(t, f)
			}
			result, err := f.execute()
			require.ErrorIs(t, err, tc.want)
			require.Equal(t, model.Trip{}, result)
		})
	}
}
