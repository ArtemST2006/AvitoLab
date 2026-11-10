package create_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ArtemST2006/AvitoLab/internal/domain/trip"
	"github.com/ArtemST2006/AvitoLab/internal/model"
	"github.com/ArtemST2006/AvitoLab/internal/usecase/trip/create"
)

type fixture struct {
	ctx   context.Context
	trips *MocktripRepository
	keys  *MockidempotencyRepository
	tx    *MocktxManager
	input model.CreateTripInput
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	return &fixture{
		ctx:   context.Background(),
		trips: NewMocktripRepository(ctrl),
		keys:  NewMockidempotencyRepository(ctrl),
		tx:    NewMocktxManager(ctrl),
		input: model.CreateTripInput{Data: model.TripData{
			UserID: uuid.New(), DriverID: uuid.New(), Price: 100,
			StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.62},
			EndPoint:   model.Coordinates{Latitude: 59.93, Longitude: 30.31},
		}},
	}
}

func (f *fixture) withKey() uuid.UUID {
	key := uuid.New()
	f.input.IdempotencyKey = &key
	return key
}

func (f *fixture) expectTx(txErr error) {
	f.tx.EXPECT().Do(f.ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
		if txErr != nil {
			return txErr
		}
		return fn(ctx)
	})
}

func (f *fixture) execute() (model.CreateTripResult, error) {
	return create.New(f.trips, f.keys, f.tx).Execute(f.ctx, f.input)
}

func (f *fixture) expectNewKey(key uuid.UUID) {
	find := f.keys.EXPECT().FindIdempotency(f.ctx, key, gomock.Any()).
		Return(model.IdempotencyRecord{}, trip.ErrIdempotencyNotFound)
	f.keys.EXPECT().ReserveIdempotency(f.ctx, key, gomock.Any(), gomock.Any()).
		After(find).Return(true, nil)
}

func bodyHash(t *testing.T, data model.TripData) string {
	t.Helper()
	body, err := json.Marshal(data)
	require.NoError(t, err)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func TestExecuteCreatesTrip(t *testing.T) {
	for _, withKey := range []bool{false, true} {
		name := "without key"
		if withKey {
			name = "with new key"
		}
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.expectTx(nil)
			if withKey {
				f.expectNewKey(f.withKey())
			}

			var saved trip.State
			insert := f.trips.EXPECT().Insert(f.ctx, gomock.Any()).
				DoAndReturn(func(_ context.Context, entity *trip.Trip) error {
					saved = entity.Snapshot()
					return nil
				})
			history := f.trips.EXPECT().AppendStatus(f.ctx, gomock.Any(), nil, trip.StatusActive, gomock.Any()).
				After(insert).DoAndReturn(func(_ context.Context, id uuid.UUID, _ *trip.Status, status trip.Status, at time.Time) error {
				require.Equal(t, saved.ID, id)
				require.Equal(t, saved.Status, status)
				require.Equal(t, saved.StartedAt, at)
				return nil
			})
			if withKey {
				f.keys.EXPECT().LinkIdempotency(f.ctx, *f.input.IdempotencyKey, gomock.Any()).
					After(history).DoAndReturn(func(_ context.Context, _, tripID uuid.UUID) error {
					require.Equal(t, saved.ID, tripID)
					return nil
				})
			}

			result, err := f.execute()
			require.NoError(t, err)
			require.False(t, result.Replayed)
			require.NotEqual(t, uuid.Nil, result.Trip.ID)
			require.Equal(t, saved.ID, result.Trip.ID)
			require.Equal(t, f.input.Data.UserID, result.Trip.UserID)
			require.Equal(t, f.input.Data.DriverID, result.Trip.DriverID)
			require.Equal(t, f.input.Data.Price, result.Trip.Price)
			require.Equal(t, trip.StatusActive, result.Trip.Status)
		})
	}
}

func TestExecuteReplaysTrip(t *testing.T) {
	for _, concurrent := range []bool{false, true} {
		name := "existing key"
		if concurrent {
			name = "concurrently reserved key"
		}
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			key := f.withKey()
			f.expectTx(nil)
			existing, err := trip.New(uuid.New(), f.input.Data.UserID, f.input.Data.DriverID,
				trip.Coordinates(f.input.Data.StartPoint), trip.Coordinates(f.input.Data.EndPoint),
				f.input.Data.Price, time.Now().UTC())
			require.NoError(t, err)
			record := model.IdempotencyRecord{BodyHash: bodyHash(t, f.input.Data), TripID: existing.Snapshot().ID}

			if concurrent {
				find := f.keys.EXPECT().FindIdempotency(f.ctx, key, gomock.Any()).
					Return(model.IdempotencyRecord{}, trip.ErrIdempotencyNotFound)
				reserve := f.keys.EXPECT().ReserveIdempotency(f.ctx, key, record.BodyHash, gomock.Any()).
					After(find).Return(false, nil)
				f.keys.EXPECT().FindIdempotency(f.ctx, key, gomock.Any()).After(reserve).Return(record, nil)
			} else {
				f.keys.EXPECT().FindIdempotency(f.ctx, key, gomock.Any()).Return(record, nil)
			}
			f.trips.EXPECT().Get(f.ctx, record.TripID).Return(existing, nil)

			result, err := f.execute()
			require.NoError(t, err)
			require.True(t, result.Replayed)
			require.Equal(t, model.FromDomain(existing), result.Trip)
		})
	}
}

func TestExecuteRejectsIdempotencyConflict(t *testing.T) {
	f := newFixture(t)
	key := f.withKey()
	f.expectTx(nil)
	f.keys.EXPECT().FindIdempotency(f.ctx, key, gomock.Any()).
		Return(model.IdempotencyRecord{BodyHash: "different request"}, nil)

	result, err := f.execute()
	require.ErrorIs(t, err, trip.ErrIdempotencyConflict)
	require.Equal(t, model.CreateTripResult{}, result)
}

func TestExecutePropagatesErrors(t *testing.T) {
	repoErr := errors.New("repository failed")
	cases := []struct {
		name   string
		keyed  bool
		want   error
		setup  func(*testing.T, *fixture)
		txFail bool
	}{
		{"driver busy", false, trip.ErrDriverBusy, func(_ *testing.T, f *fixture) {
			f.trips.EXPECT().Insert(f.ctx, gomock.Any()).Return(trip.ErrDriverBusy)
		}, false},
		{"history", false, repoErr, func(_ *testing.T, f *fixture) {
			insert := f.trips.EXPECT().Insert(f.ctx, gomock.Any()).Return(nil)
			f.trips.EXPECT().AppendStatus(f.ctx, gomock.Any(), nil, trip.StatusActive, gomock.Any()).After(insert).Return(repoErr)
		}, false},
		{"find key", true, repoErr, func(_ *testing.T, f *fixture) {
			f.keys.EXPECT().FindIdempotency(f.ctx, *f.input.IdempotencyKey, gomock.Any()).Return(model.IdempotencyRecord{}, repoErr)
		}, false},
		{"reserve key", true, repoErr, func(_ *testing.T, f *fixture) {
			find := f.keys.EXPECT().FindIdempotency(f.ctx, *f.input.IdempotencyKey, gomock.Any()).
				Return(model.IdempotencyRecord{}, trip.ErrIdempotencyNotFound)
			f.keys.EXPECT().ReserveIdempotency(f.ctx, *f.input.IdempotencyKey, gomock.Any(), gomock.Any()).After(find).Return(false, repoErr)
		}, false},
		{"read occupied key", true, repoErr, func(_ *testing.T, f *fixture) {
			find := f.keys.EXPECT().FindIdempotency(f.ctx, *f.input.IdempotencyKey, gomock.Any()).
				Return(model.IdempotencyRecord{}, trip.ErrIdempotencyNotFound)
			reserve := f.keys.EXPECT().ReserveIdempotency(f.ctx, *f.input.IdempotencyKey, gomock.Any(), gomock.Any()).After(find).Return(false, nil)
			f.keys.EXPECT().FindIdempotency(f.ctx, *f.input.IdempotencyKey, gomock.Any()).After(reserve).Return(model.IdempotencyRecord{}, repoErr)
		}, false},
		{"load replay", true, repoErr, func(t *testing.T, f *fixture) {
			id := uuid.New()
			f.keys.EXPECT().FindIdempotency(f.ctx, *f.input.IdempotencyKey, gomock.Any()).
				Return(model.IdempotencyRecord{BodyHash: bodyHash(t, f.input.Data), TripID: id}, nil)
			f.trips.EXPECT().Get(f.ctx, id).Return(nil, repoErr)
		}, false},
		{"link key", true, repoErr, func(_ *testing.T, f *fixture) {
			f.expectNewKey(*f.input.IdempotencyKey)
			insert := f.trips.EXPECT().Insert(f.ctx, gomock.Any()).Return(nil)
			history := f.trips.EXPECT().AppendStatus(f.ctx, gomock.Any(), nil, trip.StatusActive, gomock.Any()).After(insert).Return(nil)
			f.keys.EXPECT().LinkIdempotency(f.ctx, *f.input.IdempotencyKey, gomock.Any()).After(history).Return(repoErr)
		}, false},
		{"transaction", false, repoErr, nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			if tc.keyed {
				f.withKey()
			}
			if tc.txFail {
				f.expectTx(repoErr)
			} else {
				f.expectTx(nil)
				tc.setup(t, f)
			}
			result, err := f.execute()
			require.ErrorIs(t, err, tc.want)
			require.Equal(t, model.CreateTripResult{}, result)
		})
	}
}

func TestExecuteRejectsInvalidTripBeforeTransaction(t *testing.T) {
	f := newFixture(t)
	f.input.Data.DriverID = uuid.Nil
	result, err := f.execute()
	var validationErr *trip.ValidationError
	require.ErrorAs(t, err, &validationErr)
	require.Equal(t, "driver_id", validationErr.Field)
	require.Equal(t, model.CreateTripResult{}, result)
}

func TestCreatedMetricOnlyAfterCommitAndNotOnReplay(t *testing.T) {
	for _, mode := range []string{"created", "replayed", "rolled back"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			metrics := NewMocktripMetrics(gomock.NewController(t))
			if mode == "replayed" {
				key := f.withKey()
				entity, err := trip.New(uuid.New(), f.input.Data.UserID, f.input.Data.DriverID,
					trip.Coordinates(f.input.Data.StartPoint), trip.Coordinates(f.input.Data.EndPoint), f.input.Data.Price, time.Now())
				require.NoError(t, err)
				f.keys.EXPECT().FindIdempotency(f.ctx, key, gomock.Any()).Return(
					model.IdempotencyRecord{BodyHash: bodyHash(t, f.input.Data), TripID: entity.Snapshot().ID}, nil,
				)
				f.trips.EXPECT().Get(f.ctx, entity.Snapshot().ID).Return(entity, nil)
			} else {
				f.trips.EXPECT().Insert(f.ctx, gomock.Any()).Return(nil)
				if mode == "created" {
					f.trips.EXPECT().AppendStatus(f.ctx, gomock.Any(), nil, trip.StatusActive, gomock.Any()).Return(nil)
					metrics.EXPECT().Created(f.ctx)
				} else {
					f.trips.EXPECT().AppendStatus(f.ctx, gomock.Any(), nil, trip.StatusActive, gomock.Any()).Return(errors.New("rollback"))
				}
			}
			f.expectTx(nil)
			_, err := create.New(f.trips, f.keys, f.tx, metrics).Execute(f.ctx, f.input)
			if mode == "rolled back" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
