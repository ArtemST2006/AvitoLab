package ready_test

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/ArtemST2006/AvitoLab/internal/http/ready"
)

func TestHandleReady(t *testing.T) {
	tests := []struct {
		name       string
		pingErr    error
		wantStatus int
		wantBody   api.HealthResponseStatus
	}{
		{"database is ready", nil, http.StatusOK, api.Ok},
		{"database is unavailable", errors.New("database password=secret"), http.StatusServiceUnavailable, api.Unavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := NewMockdatabase(gomock.NewController(t))
			db.EXPECT().Ping(gomock.Any()).Return(tt.pingErr)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			handler := ready.New(db, logger, time.Second)
			request := httptest.NewRequest(http.MethodGet, "/ready", nil)
			response := httptest.NewRecorder()

			handler.Handle(response, request)

			require.Equal(t, tt.wantStatus, response.Code)
			var body api.HealthResponse
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			require.Equal(t, tt.wantBody, body.Status)
			require.NotContains(t, response.Body.String(), "password=secret")
		})
	}
}
