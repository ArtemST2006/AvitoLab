package get_test

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
	"github.com/ArtemST2006/AvitoLab/internal/usecase/trip/get"
)

func TestExecute(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	entity, err := trip.New(id, uuid.New(), uuid.New(),
		trip.Coordinates{Latitude: 55.75, Longitude: 37.62},
		trip.Coordinates{Latitude: 59.93, Longitude: 30.31},
		100, time.Now())
	require.NoError(t, err)
	repoErr := errors.New("database unavailable")

	for _, tc := range []struct {
		name   string
		entity *trip.Trip
		err    error
	}{
		{name: "success", entity: entity},
		{name: "not found", err: trip.ErrTripNotFound},
		{name: "repository error", err: repoErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := NewMocktripRepository(gomock.NewController(t))
			repo.EXPECT().Get(ctx, id).Return(tc.entity, tc.err)

			got, err := get.New(repo).Execute(ctx, id)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				require.Equal(t, model.Trip{}, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, model.FromDomain(entity), got)
		})
	}
}
