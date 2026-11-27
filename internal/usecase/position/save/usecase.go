package save

import (
	"context"

	"github.com/ArtemST2006/AvitoLab/internal/domain/trip"
	"github.com/ArtemST2006/AvitoLab/internal/model"
)

type UseCase struct {
	trips     tripRepository
	positions positionRepository
	tx        txManager
}

// New constructs the package component.
func New(position positionRepository, trips tripRepository, tx txManager) *UseCase {
	u := &UseCase{trips: trips, positions: position, tx: tx}
	return u
}

func (u *UseCase) Execute(ctx context.Context, input model.SavePositionInput) (model.Position, error) {
	var position model.Position

	err := u.tx.Do(ctx, func(ctx context.Context) error {
		entity, err := u.trips.GetForUpdate(ctx, input.TripID)
		if err != nil {
			return err
		}
		if entity.Snapshot().Status != trip.StatusActive {
			return trip.ErrTripCompleted
		}

		position, err = u.positions.Insert(ctx, input)
		if err != nil {
			return err
		}

		return u.trips.AdvanceLastPositionAt(ctx, input.TripID, input.RecordedAt)
	})
	if err != nil {
		return model.Position{}, err
	}

	return position, nil
}
