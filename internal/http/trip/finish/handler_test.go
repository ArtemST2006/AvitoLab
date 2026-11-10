package finish_test

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
	"github.com/ArtemST2006/AvitoLab/internal/http/trip/finish"
	"github.com/ArtemST2006/AvitoLab/internal/model"
)

func TestHandleFinishTrip(t *testing.T) {
	id := uuid.New()
	finishedAt := time.Date(2026, time.November, 9, 12, 0, 0, 0, time.UTC)
	completed := model.Trip{
		ID: id, UserID: uuid.New(), DriverID: uuid.New(),
		Status: trip.StatusCompleted, StartedAt: finishedAt.Add(-time.Hour), FinishedAt: &finishedAt,
	}
	tests := []struct {
		name       string
		result     model.Trip
		err        error
		wantStatus int
		wantCode   string
		wantDetail string
	}{
		{
			name: "success", result: completed, wantStatus: http.StatusOK,
		},
		{
			name: "already completed", err: trip.ErrTripCompleted,
			wantStatus: http.StatusConflict, wantCode: "trip_completed",
			wantDetail: trip.ErrTripCompleted.Error(),
		},
		{
			name: "trip not found", err: trip.ErrTripNotFound,
			wantStatus: http.StatusNotFound, wantCode: "trip_not_found",
			wantDetail: trip.ErrTripNotFound.Error(),
		},
		{
			name: "repository error", err: errors.New("database password=secret"),
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usecase := NewMockusecase(gomock.NewController(t))
			usecase.EXPECT().Execute(gomock.Any(), id).Return(tt.result, tt.err)
			var logOutput bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logOutput, nil))
			handler := finish.New(usecase, logger, time.Second)
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/trips/"+id.String()+"/finish", nil)
			response := httptest.NewRecorder()

			handler.Handle(response, request, id)

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
			if tt.err == nil {
				var body api.Trip
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
				require.Equal(t, completed.ID, body.Id)
				require.Equal(t, api.TripStatus(trip.StatusCompleted), body.Status)
				require.NotNil(t, body.FinishedAt)
				require.Equal(t, finishedAt, *body.FinishedAt)
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
			require.Equal(t, tt.wantDetail, *body.Detail)
		})
	}
}
