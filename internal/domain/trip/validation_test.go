package trip_test

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ArtemST2006/AvitoLab/internal/domain/trip"
)

const (
	latitudeField   = "latitude"
	longitudeField  = "longitude"
	latitudeReason  = "must be between -90 and 90"
	longitudeReason = "must be between -180 and 180"
)

func validState() trip.State {
	return trip.State{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: trip.Coordinates{Latitude: 55.75, Longitude: 37.62},
		EndPoint:   trip.Coordinates{Latitude: 59.93, Longitude: 30.32},
		Price:      100,
		Status:     trip.StatusActive,
		StartedAt:  time.Date(2026, time.November, 9, 10, 0, 0, 0, time.UTC),
	}
}

func TestNewInitializesActiveTrip(t *testing.T) {
	tests := []struct {
		name       string
		start, end trip.Coordinates
		price      int64
	}{
		{
			name:  "ordinary coordinates and price",
			start: trip.Coordinates{Latitude: 55.75, Longitude: 37.62},
			end:   trip.Coordinates{Latitude: 59.93, Longitude: 30.32},
			price: 100,
		},
		{
			name:  "minimum coordinates and zero price",
			start: trip.Coordinates{Latitude: -90, Longitude: -180},
			end:   trip.Coordinates{},
		},
		{
			name:  "maximum coordinates",
			start: trip.Coordinates{},
			end:   trip.Coordinates{Latitude: 90, Longitude: 180},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, userID, driverID := uuid.New(), uuid.New(), uuid.New()
			startedAt := time.Date(2026, time.November, 9, 10, 0, 0, 0, time.UTC)

			journey, err := trip.New(id, userID, driverID, tt.start, tt.end, tt.price, startedAt)

			require.NoError(t, err)
			require.NotNil(t, journey)
			assert.Equal(t, trip.State{
				ID: id, UserID: userID, DriverID: driverID,
				StartPoint: tt.start, EndPoint: tt.end, Price: tt.price,
				Status: trip.StatusActive, StartedAt: startedAt,
			}, journey.Snapshot())
		})
	}
}

func TestNewRejectsInvalidTrip(t *testing.T) {
	tests := []struct {
		name      string
		change    func(*trip.State)
		wantField string
	}{
		{"nil user", func(s *trip.State) { s.UserID = uuid.Nil }, "user_id"},
		{"nil driver", func(s *trip.State) { s.DriverID = uuid.Nil }, "driver_id"},
		{"invalid start point", func(s *trip.State) { s.StartPoint.Latitude = 91 }, "start_point.latitude"},
		{"negative price", func(s *trip.State) { s.Price = -1 }, "price"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := validState()
			tt.change(&state)

			journey, err := trip.New(
				state.ID, state.UserID, state.DriverID,
				state.StartPoint, state.EndPoint, state.Price, state.StartedAt,
			)

			require.Nil(t, journey)
			var validationErr *trip.ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Equal(t, tt.wantField, validationErr.Field)
		})
	}
}

func TestRestoreAcceptsValidStates(t *testing.T) {
	finishedAt := time.Date(2026, time.November, 9, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		change func(*trip.State)
	}{
		{"active", func(*trip.State) {}},
		{"completed", func(s *trip.State) {
			s.Status = trip.StatusCompleted
			s.FinishedAt = &finishedAt
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := validState()
			tt.change(&state)

			journey, err := trip.Restore(state)

			require.NoError(t, err)
			require.NotNil(t, journey)
			assert.Equal(t, state, journey.Snapshot())
		})
	}
}

func TestRestoreRejectsInvalidState(t *testing.T) {
	tests := []struct {
		name, wantField, wantReason string
		change                      func(*trip.State)
	}{
		{"nil user", "user_id", "must not be nil UUID", func(s *trip.State) { s.UserID = uuid.Nil }},
		{"nil driver", "driver_id", "must not be nil UUID", func(s *trip.State) { s.DriverID = uuid.Nil }},
		{"negative price", "price", "must be non-negative", func(s *trip.State) { s.Price = -1 }},
		{"active with finish time", "finished_at", "active trip cannot be finished", func(s *trip.State) {
			at := s.StartedAt.Add(time.Hour)
			s.FinishedAt = &at
		}},
		{"completed without finish time", "finished_at", "completed trip requires finish time", func(s *trip.State) {
			s.Status = trip.StatusCompleted
		}},
		{"unknown status", "status", "unknown trip status", func(s *trip.State) { s.Status = "cancelled" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := validState()
			tt.change(&state)

			journey, err := trip.Restore(state)

			require.Nil(t, journey)
			var validationErr *trip.ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Equal(t, tt.wantField, validationErr.Field)
			assert.Equal(t, tt.wantReason, validationErr.Reason)
			assert.Equal(t, tt.wantField+": "+tt.wantReason, err.Error())
		})
	}
}

func TestRestoreRejectsInvalidCoordinates(t *testing.T) {
	tests := []struct {
		name, field, reason string
		point               trip.Coordinates
	}{
		{"latitude below minimum", latitudeField, latitudeReason, trip.Coordinates{Latitude: -90.1}},
		{"latitude above maximum", latitudeField, latitudeReason, trip.Coordinates{Latitude: 90.1}},
		{"latitude NaN", latitudeField, latitudeReason, trip.Coordinates{Latitude: math.NaN()}},
		{"latitude positive infinity", latitudeField, latitudeReason, trip.Coordinates{Latitude: math.Inf(1)}},
		{"latitude negative infinity", latitudeField, latitudeReason, trip.Coordinates{Latitude: math.Inf(-1)}},
		{"longitude below minimum", longitudeField, longitudeReason, trip.Coordinates{Longitude: -180.1}},
		{"longitude above maximum", longitudeField, longitudeReason, trip.Coordinates{Longitude: 180.1}},
		{"longitude NaN", longitudeField, longitudeReason, trip.Coordinates{Longitude: math.NaN()}},
		{"longitude positive infinity", longitudeField, longitudeReason, trip.Coordinates{Longitude: math.Inf(1)}},
		{"longitude negative infinity", longitudeField, longitudeReason, trip.Coordinates{Longitude: math.Inf(-1)}},
	}
	targets := []struct {
		name   string
		change func(*trip.State, trip.Coordinates)
	}{
		{"start_point", func(s *trip.State, point trip.Coordinates) { s.StartPoint = point }},
		{"end_point", func(s *trip.State, point trip.Coordinates) { s.EndPoint = point }},
	}
	for _, target := range targets {
		t.Run(target.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					state := validState()
					target.change(&state, tt.point)

					journey, err := trip.Restore(state)

					require.Nil(t, journey)
					var validationErr *trip.ValidationError
					require.ErrorAs(t, err, &validationErr)
					assert.Equal(t, target.name+"."+tt.field, validationErr.Field)
					assert.Equal(t, tt.reason, validationErr.Reason)
				})
			}
		})
	}
}
