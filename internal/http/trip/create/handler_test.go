package create_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ArtemST2006/AvitoLab/internal/domain/trip"
	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/ArtemST2006/AvitoLab/internal/http/trip/create"
	"github.com/ArtemST2006/AvitoLab/internal/model"
)

func TestHandleCreateTrip(t *testing.T) {
	const invalidRequestCode = "invalid_request"
	id, userID, driverID, key := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	data := api.TripData{
		UserId: userID, DriverId: driverID,
		StartPoint: api.Coordinates{Latitude: 55.75, Longitude: 37.62},
		EndPoint:   api.Coordinates{Latitude: 59.93, Longitude: 30.32},
		Price:      100,
	}
	validBody, err := json.Marshal(data)
	require.NoError(t, err)
	missingUser := data
	missingUser.UserId = uuid.Nil
	missingUserBody, err := json.Marshal(missingUser)
	require.NoError(t, err)
	missingDriver := data
	missingDriver.DriverId = uuid.Nil
	missingDriverBody, err := json.Marshal(missingDriver)
	require.NoError(t, err)

	wantInput := model.CreateTripInput{
		Data: model.TripData{
			UserID: userID, DriverID: driverID,
			StartPoint: model.Coordinates(data.StartPoint),
			EndPoint:   model.Coordinates(data.EndPoint),
			Price:      data.Price,
		},
		IdempotencyKey: &key,
	}
	created := model.Trip{
		ID: id, UserID: userID, DriverID: driverID,
		StartPoint: wantInput.Data.StartPoint, EndPoint: wantInput.Data.EndPoint,
		Price: data.Price, Status: trip.StatusActive,
		StartedAt: time.Date(2026, time.November, 9, 10, 0, 0, 0, time.UTC),
	}
	tests := []struct {
		name          string
		body          []byte
		result        model.CreateTripResult
		err           error
		expectExecute bool
		wantStatus    int
		wantCode      string
		wantDetail    string
	}{
		{
			name: "created", body: validBody,
			result: model.CreateTripResult{Trip: created}, expectExecute: true,
			wantStatus: http.StatusCreated,
		},
		{
			name: "idempotent replay", body: validBody,
			result: model.CreateTripResult{Trip: created, Replayed: true}, expectExecute: true,
			wantStatus: http.StatusOK,
		},
		{
			name: "driver busy", body: validBody, err: trip.ErrDriverBusy,
			expectExecute: true, wantStatus: http.StatusConflict, wantCode: "driver_busy",
			wantDetail: trip.ErrDriverBusy.Error(),
		},
		{
			name: "idempotency conflict", body: validBody, err: trip.ErrIdempotencyConflict,
			expectExecute: true, wantStatus: http.StatusConflict, wantCode: "idempotency_conflict",
			wantDetail: trip.ErrIdempotencyConflict.Error(),
		},
		{
			name: "repository error", body: validBody, err: errors.New("database password=secret"),
			expectExecute: true, wantStatus: http.StatusInternalServerError, wantCode: "internal_error",
		},
		{
			name: "malformed JSON", body: []byte("{"),
			wantStatus: http.StatusBadRequest, wantCode: invalidRequestCode,
		},
		{
			name: "missing user UUID", body: missingUserBody,
			wantStatus: http.StatusBadRequest, wantCode: invalidRequestCode,
			wantDetail: "user_id must not be nil UUID",
		},
		{
			name: "missing driver UUID", body: missingDriverBody,
			wantStatus: http.StatusBadRequest, wantCode: invalidRequestCode,
			wantDetail: "driver_id must not be nil UUID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usecase := NewMockusecase(gomock.NewController(t))
			if tt.expectExecute {
				usecase.EXPECT().Execute(gomock.Any(), wantInput).Return(tt.result, tt.err)
			}
			var logOutput bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logOutput, nil))
			handler := create.New(usecase, logger, time.Second)
			endpoint := create.ValidateBody(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handler.Handle(w, r, api.CreateTripParams{IdempotencyKey: &key})
			}))
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/trips", bytes.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()

			endpoint.ServeHTTP(response, request)

			if tt.err != nil {
				var entry map[string]any
				require.NoError(t, json.Unmarshal(bytes.TrimSpace(logOutput.Bytes()), &entry))
				wantLevel := "INFO"
				if tt.wantStatus == http.StatusInternalServerError {
					wantLevel = "ERROR"
				}
				require.Equal(t, wantLevel, entry["level"])
				require.Equal(t, tt.err.Error(), entry["error"])
			}

			require.Equal(t, tt.wantStatus, response.Code)
			if tt.wantCode == "" {
				require.Equal(t, "/api/v1/trips/"+id.String(), response.Header().Get("Location"))
				var body api.Trip
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
				require.Equal(t, created.ID, body.Id)
				require.Equal(t, created.UserID, body.UserId)
				require.Equal(t, created.DriverID, body.DriverId)
				require.Equal(t, api.TripStatus(trip.StatusActive), body.Status)
				return
			}

			require.Equal(t, "application/problem+json", response.Header().Get("Content-Type"))
			var body api.Problem
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			require.EqualValues(t, tt.wantStatus, body.Status)
			require.Equal(t, tt.wantCode, body.Code)
			require.NotNil(t, body.Instance)
			require.Equal(t, request.URL.Path, *body.Instance)
			if tt.wantStatus == http.StatusInternalServerError {
				require.Nil(t, body.Detail)
				require.NotContains(t, response.Body.String(), "password=secret")
				return
			}
			require.NotNil(t, body.Detail)
			if tt.wantDetail != "" {
				require.Equal(t, tt.wantDetail, *body.Detail)
			}
		})
	}
}
