package save

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/ArtemST2006/AvitoLab/internal/domain/trip"
	"github.com/ArtemST2006/AvitoLab/internal/model"
)

type positionRepository interface {
	Insert(context.Context, model.SavePositionInput) (model.Position, error)
}

type tripRepository interface {
	GetForUpdate(context.Context, uuid.UUID) (*trip.Trip, error)
	AdvanceLastPositionAt(context.Context, uuid.UUID, time.Time) error
}

type txManager interface {
	Do(context.Context, func(context.Context) error) error
}
