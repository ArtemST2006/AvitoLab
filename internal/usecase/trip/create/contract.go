// Package create implements the create-trip use case.
package create

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/ArtemST2006/AvitoLab/internal/domain/trip"
	"github.com/ArtemST2006/AvitoLab/internal/model"
)

//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=create_test
type tripRepository interface {
	Insert(context.Context, *trip.Trip) error
	AppendStatus(context.Context, uuid.UUID, *trip.Status, trip.Status, time.Time) error
	Get(context.Context, uuid.UUID) (*trip.Trip, error)
}

type idempotencyRepository interface {
	FindIdempotency(context.Context, uuid.UUID, time.Time) (model.IdempotencyRecord, error)
	ReserveIdempotency(context.Context, uuid.UUID, string, time.Time) (bool, error)
	LinkIdempotency(context.Context, uuid.UUID, uuid.UUID) error
}

type txManager interface {
	Do(context.Context, func(context.Context) error) error
}
