package save

import (
	"context"

	"github.com/google/uuid"

	"github.com/ArtemST2006/AvitoLab/internal/model"
)

//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=save_test
type usecase interface {
	Execute(context.Context, model.SavePositionInput) (model.Position, error)
}

type positionQueue interface {
	Cancel(uuid.UUID)
}

type positionMetrics interface {
	Saved(context.Context)
}
