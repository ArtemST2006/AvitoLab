// Package get implements the get-trip use case.
package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/ArtemST2006/AvitoLab/internal/model"
)

// UseCase coordinates this trip operation.
type UseCase struct {
	trips tripRepository
}

// New constructs the package component.
func New(trips tripRepository) *UseCase {
	return &UseCase{trips: trips}
}

// Execute runs this trip operation.
func (u *UseCase) Execute(ctx context.Context, id uuid.UUID) (model.Trip, error) {
	entity, err := u.trips.Get(ctx, id)
	if err != nil {
		return model.Trip{}, err
	}
	return model.FromDomain(entity), nil
}
