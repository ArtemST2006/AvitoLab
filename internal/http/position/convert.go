// Package position converts application positions to API responses.
package position

import (
	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/ArtemST2006/AvitoLab/internal/model"
)

// ToAPI converts an application position to the OpenAPI response type.
func ToAPI(value model.Position) api.TripPosition {
	return api.TripPosition{
		Id:          value.ID,
		TripId:      value.TripID,
		Coordinates: api.Coordinates(value.Coordinates),
		RecordedAt:  value.RecordedAt,
	}
}
