// Package create serves the create-trip endpoint.
package create

import (
	"context"

	"github.com/ArtemST2006/AvitoLab/internal/model"
)

//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=create_test
type usecase interface {
	Execute(context.Context, model.CreateTripInput) (model.CreateTripResult, error)
}
