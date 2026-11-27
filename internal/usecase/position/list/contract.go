// Package list implements the list-trip-positions use case.
package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/ArtemST2006/AvitoLab/internal/domain/trip"
	"github.com/ArtemST2006/AvitoLab/internal/model"
)

type tripRepository interface {
	Get(context.Context, uuid.UUID) (*trip.Trip, error)
}

type positionRepository interface {
	ListByTripID(context.Context, uuid.UUID) ([]model.Position, error)
}
