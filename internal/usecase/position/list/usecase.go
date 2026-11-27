package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/ArtemST2006/AvitoLab/internal/model"
)

// UseCase reads the route for an existing trip.
type UseCase struct {
	trips     tripRepository
	positions positionRepository
}

// New constructs the package component.
func New(trips tripRepository, positions positionRepository) *UseCase {
	return &UseCase{trips: trips, positions: positions}
}

// Execute returns all positions of a trip in route order.
func (u *UseCase) Execute(ctx context.Context, id uuid.UUID) ([]model.Position, error) {
	if _, err := u.trips.Get(ctx, id); err != nil {
		return nil, err
	}
	return u.positions.ListByTripID(ctx, id)
}
