// Package list serves the list-trip-positions endpoint.
package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/ArtemST2006/AvitoLab/internal/model"
)

type usecase interface {
	Execute(context.Context, uuid.UUID) ([]model.Position, error)
}
