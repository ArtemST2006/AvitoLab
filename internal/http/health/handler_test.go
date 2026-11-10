package health_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/ArtemST2006/AvitoLab/internal/http/health"
)

func TestHandleHealth(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	health.New().Handle(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	var body api.HealthResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, api.Ok, body.Status)
}
