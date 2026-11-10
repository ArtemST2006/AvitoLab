package get_test

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
	"github.com/ArtemST2006/AvitoLab/internal/http/trip/get"
	"github.com/ArtemST2006/AvitoLab/internal/model"
)

func TestHandleGetTrip(t *testing.T) {
	id := uuid.New()
	want := model.Trip{
		ID:        id,
		UserID:    uuid.New(),
		DriverID:  uuid.New(),
		Status:    trip.StatusActive,
		StartedAt: time.Date(2026, time.November, 9, 10, 0, 0, 0, time.UTC),
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
			name:       "success",
			result:     want,
			wantStatus: http.StatusOK,
		},
		{
			name:       "trip not found",
			err:        trip.ErrTripNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
			wantDetail: "trip not found",
		},
		{
			name:       "repository error",
			err:        errors.New("database password=secret: connection failed"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			usecase := NewMockusecase(controller)
			usecase.EXPECT().Execute(gomock.Any(), id).Return(tt.result, tt.err)

			var logOutput bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logOutput, nil))
			handler := get.New(usecase, logger, time.Second)
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/trips/"+id.String(), nil)
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
				var body struct {
					ID       uuid.UUID   `json:"id"`
					UserID   uuid.UUID   `json:"user_id"`
					DriverID uuid.UUID   `json:"driver_id"`
					Status   trip.Status `json:"status"`
				}
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
				require.Equal(t, want.ID, body.ID)
				require.Equal(t, want.UserID, body.UserID)
				require.Equal(t, want.DriverID, body.DriverID)
				require.Equal(t, want.Status, body.Status)
				return
			}

			require.Equal(t, "application/problem+json", response.Header().Get("Content-Type"))
			var body api.Problem
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			require.EqualValues(t, tt.wantStatus, body.Status)
			require.Equal(t, tt.wantCode, body.Code)
			require.NotNil(t, body.Instance)
			require.Equal(t, request.URL.Path, *body.Instance)
			if tt.wantDetail == "" {
				require.Nil(t, body.Detail)
				require.NotContains(t, response.Body.String(), "password=secret")
				return
			}
			require.NotNil(t, body.Detail)
			require.Equal(t, tt.wantDetail, *body.Detail)
		})
	}
}
