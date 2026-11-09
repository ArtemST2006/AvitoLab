// Package finish serves the finish-trip endpoint.
package finish

import (
	"context"

	"github.com/google/uuid"

	"github.com/ArtemST2006/AvitoLab/internal/model"
)

//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=finish_test
type usecase interface {
	Execute(context.Context, uuid.UUID) (model.Trip, error)
}
