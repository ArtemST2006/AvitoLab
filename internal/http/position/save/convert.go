package save

import (
	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/ArtemST2006/AvitoLab/internal/model"
)

func toInput(input api.PositionData, id api.TripId) model.SavePositionInput {
	return model.SavePositionInput{
		TripID:      id,
		Coordinates: model.Coordinates(input.Coordinates),
		RecordedAt:  input.RecordedAt,
	}
}
