// Package model defines data exchanged between application layers.
package model

import (
	"time"

	"github.com/google/uuid"

	"github.com/ArtemST2006/AvitoLab/internal/domain/trip"
)

// Coordinates is a latitude and longitude pair.
type Coordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// TripData has the same canonical JSON field order as the OpenAPI request.
// Its encoding is used for the persisted idempotency body hash.
type TripData struct {
	DriverID   uuid.UUID   `json:"driver_id"`
	EndPoint   Coordinates `json:"end_point"`
	Price      int64       `json:"price"`
	StartPoint Coordinates `json:"start_point"`
	UserID     uuid.UUID   `json:"user_id"`
}

// CreateTripInput contains validated create-trip data.
type CreateTripInput struct {
	Data           TripData
	IdempotencyKey *uuid.UUID
}

// CreateTripResult includes the trip and replay status.
type CreateTripResult struct {
	Trip     Trip
	Replayed bool
}

// Trip contains trip data exchanged across application layers.
type Trip struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	DriverID       uuid.UUID
	StartPoint     Coordinates
	EndPoint       Coordinates
	Price          int64
	Status         trip.Status
	StartedAt      time.Time
	FinishedAt     *time.Time
	LastPositionAt *time.Time
}

// IdempotencyRecord describes a reserved request key.
type IdempotencyRecord struct {
	Key       uuid.UUID
	BodyHash  string
	TripID    uuid.UUID
	CreatedAt time.Time
}

// FromDomain copies a domain trip into an application result.
func FromDomain(entity *trip.Trip) Trip {
	state := entity.Snapshot()
	return Trip{
		ID: state.ID, UserID: state.UserID, DriverID: state.DriverID,
		StartPoint: Coordinates(state.StartPoint), EndPoint: Coordinates(state.EndPoint),
		Price: state.Price, Status: state.Status, StartedAt: state.StartedAt,
		FinishedAt: state.FinishedAt, LastPositionAt: state.LastPositionAt,
	}
}
