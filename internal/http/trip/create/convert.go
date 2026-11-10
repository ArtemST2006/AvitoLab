// Package create serves the create-trip endpoint.
package create

import (
	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/ArtemST2006/AvitoLab/internal/model"
)

func toInput(data api.TripData, key *api.IdempotencyKey) model.CreateTripInput {
	return model.CreateTripInput{
		Data: model.TripData{
			DriverID: data.DriverId, EndPoint: model.Coordinates(data.EndPoint),
			Price: data.Price, StartPoint: model.Coordinates(data.StartPoint),
			UserID: data.UserId,
		},
		IdempotencyKey: key,
	}
}
