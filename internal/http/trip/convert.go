// Package trip converts application trip data to API responses.
package trip

import (
	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/ArtemST2006/AvitoLab/internal/model"
)

// ToAPI converts an application trip to the OpenAPI response type.
func ToAPI(value model.Trip) api.Trip {
	return api.Trip{
		Id: value.ID, UserId: value.UserID, DriverId: value.DriverID,
		StartPoint: api.Coordinates(value.StartPoint),
		EndPoint:   api.Coordinates(value.EndPoint),
		Price:      value.Price, Status: api.TripStatus(value.Status),
		StartedAt: value.StartedAt, FinishedAt: value.FinishedAt,
		LastPositionAt: value.LastPositionAt,
	}
}
