// Package trip defines trip state and business rules.
package trip

import (
	"math"
	"time"

	"github.com/google/uuid"
)

// Status is a trip lifecycle state.
type Status string

// StatusActive and StatusCompleted are supported trip states.
const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
)

// Coordinates is a latitude and longitude pair.
type Coordinates struct {
	Latitude  float64
	Longitude float64
}

// State is a copy of the entity's data for persistence and presentation.
type State struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	DriverID       uuid.UUID
	StartPoint     Coordinates
	EndPoint       Coordinates
	Price          int64
	Status         Status
	StartedAt      time.Time
	FinishedAt     *time.Time
	LastPositionAt *time.Time
}

// Trip owns its state transitions. Callers can only obtain a copy of its state.
type Trip struct {
	state State
}

// New constructs the package component.
func New(id, userID, driverID uuid.UUID, start, end Coordinates, price int64, at time.Time) (*Trip, error) {
	return Restore(State{
		ID: id, UserID: userID, DriverID: driverID,
		StartPoint: start, EndPoint: end, Price: price,
		Status: StatusActive, StartedAt: at,
	})
}

// Restore reconstructs a persisted trip and checks the state invariant.
func Restore(state State) (*Trip, error) {
	if state.UserID == uuid.Nil {
		return nil, &ValidationError{Field: "user_id", Reason: "must not be nil UUID"}
	}
	if state.DriverID == uuid.Nil {
		return nil, &ValidationError{Field: "driver_id", Reason: "must not be nil UUID"}
	}
	if err := validateCoordinates("start_point", state.StartPoint); err != nil {
		return nil, err
	}
	if err := validateCoordinates("end_point", state.EndPoint); err != nil {
		return nil, err
	}
	if state.Price < 0 {
		return nil, &ValidationError{Field: "price", Reason: "must be non-negative"}
	}
	switch state.Status {
	case StatusActive:
		if state.FinishedAt != nil {
			return nil, &ValidationError{Field: "finished_at", Reason: "active trip cannot be finished"}
		}
	case StatusCompleted:
		if state.FinishedAt == nil {
			return nil, &ValidationError{Field: "finished_at", Reason: "completed trip requires finish time"}
		}
	default:
		return nil, &ValidationError{Field: "status", Reason: "unknown trip status"}
	}
	state.FinishedAt = copyTime(state.FinishedAt)
	state.LastPositionAt = copyTime(state.LastPositionAt)
	return &Trip{state: state}, nil
}

// Snapshot returns an independent copy of the trip state.
func (t *Trip) Snapshot() State {
	state := t.state
	state.FinishedAt = copyTime(state.FinishedAt)
	state.LastPositionAt = copyTime(state.LastPositionAt)
	return state
}

// Finish moves an active trip to the completed state.
func (t *Trip) Finish(at time.Time) error {
	if t.state.Status != StatusActive {
		return ErrTripCompleted
	}
	at = at.UTC()
	t.state.Status = StatusCompleted
	t.state.FinishedAt = &at
	return nil
}

func validateCoordinates(field string, point Coordinates) error {
	if math.IsNaN(point.Latitude) || math.IsInf(point.Latitude, 0) || point.Latitude < -90 || point.Latitude > 90 {
		return &ValidationError{Field: field + ".latitude", Reason: "must be between -90 and 90"}
	}
	if math.IsNaN(point.Longitude) || math.IsInf(point.Longitude, 0) || point.Longitude < -180 || point.Longitude > 180 {
		return &ValidationError{Field: field + ".longitude", Reason: "must be between -180 and 180"}
	}
	return nil
}

func copyTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
