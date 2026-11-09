// Package get implements the get-trip use case.
package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/ArtemST2006/AvitoLab/internal/domain/trip"
)

//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=get_test
type tripRepository interface {
	Get(context.Context, uuid.UUID) (*trip.Trip, error)
}
