package domainerror_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ArtemST2006/AvitoLab/internal/domain/trip"
	"github.com/ArtemST2006/AvitoLab/internal/http/domainerror"
)

func TestProblem(t *testing.T) {
	const tripNotFoundText = "trip not found"
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantType   string
		wantTitle  string
		wantDetail string
	}{
		{
			name:       "validation error",
			err:        &trip.ValidationError{Field: "price", Reason: "must be non-negative"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
			wantType:   "https://tripgo.example/problems/invalid-request",
			wantTitle:  "Invalid request",
			wantDetail: "price: must be non-negative",
		},
		{
			name:       "wrapped validation error",
			err:        fmt.Errorf("create trip: %w", &trip.ValidationError{Field: "user_id", Reason: "must not be nil UUID"}),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
			wantType:   "https://tripgo.example/problems/invalid-request",
			wantTitle:  "Invalid request",
			wantDetail: "user_id: must not be nil UUID",
		},
		{
			name:       tripNotFoundText,
			err:        trip.ErrTripNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
			wantType:   "https://tripgo.example/problems/trip-not-found",
			wantTitle:  "Trip not found",
			wantDetail: tripNotFoundText,
		},
		{
			name:       "wrapped trip not found",
			err:        fmt.Errorf("%w", trip.ErrTripNotFound),
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
			wantType:   "https://tripgo.example/problems/trip-not-found",
			wantTitle:  "Trip not found",
			wantDetail: tripNotFoundText,
		},
		{
			name:       "trip already completed",
			err:        trip.ErrTripCompleted,
			wantStatus: http.StatusConflict,
			wantCode:   "trip_completed",
			wantType:   "https://tripgo.example/problems/trip-completed",
			wantTitle:  "Trip completed",
			wantDetail: "trip already completed",
		},
		{
			name:       "driver busy",
			err:        trip.ErrDriverBusy,
			wantStatus: http.StatusConflict,
			wantCode:   "driver_busy",
			wantType:   "https://tripgo.example/problems/driver-busy",
			wantTitle:  "Driver busy",
			wantDetail: "driver already has an active trip",
		},
		{
			name:       "idempotency conflict",
			err:        trip.ErrIdempotencyConflict,
			wantStatus: http.StatusConflict,
			wantCode:   "idempotency_conflict",
			wantType:   "https://tripgo.example/problems/idempotency-conflict",
			wantTitle:  "Idempotency conflict",
			wantDetail: "idempotency key reused with different body",
		},
		{
			name:       "repository error does not leak details",
			err:        errors.New("database password=secret: connection failed"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
			wantType:   "https://tripgo.example/problems/internal-error",
			wantTitle:  "Internal server error",
		},
		{
			name:       "internal idempotency error does not leak details",
			err:        trip.ErrIdempotencyNotFound,
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
			wantType:   "https://tripgo.example/problems/internal-error",
			wantTitle:  "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := domainerror.Problem(tt.err)

			assert.Equal(t, tt.wantStatus, status)
			assert.EqualValues(t, tt.wantStatus, body.Status)
			assert.Equal(t, tt.wantCode, body.Code)
			assert.Equal(t, tt.wantType, body.Type)
			assert.Equal(t, tt.wantTitle, body.Title)
			assert.Nil(t, body.Instance)
			if tt.wantDetail == "" {
				assert.Nil(t, body.Detail)
				return
			}
			require.NotNil(t, body.Detail)
			assert.Equal(t, tt.wantDetail, *body.Detail)
		})
	}
}
