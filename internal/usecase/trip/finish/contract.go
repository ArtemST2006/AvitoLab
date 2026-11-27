// Package finish implements the finish-trip use case.
package finish

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/ArtemST2006/AvitoLab/internal/domain/trip"
)

//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=finish_test
type tripRepository interface {
	GetForUpdate(context.Context, uuid.UUID) (*trip.Trip, error)
	Update(context.Context, *trip.Trip) error
	AppendStatus(context.Context, uuid.UUID, *trip.Status, trip.Status, time.Time) error
}

type txManager interface {
	Do(context.Context, func(context.Context) error) error
}

type tripMetrics interface {
	Completed(context.Context)
}
