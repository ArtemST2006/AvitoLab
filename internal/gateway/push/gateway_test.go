package push

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGatewayRequestsPosition(t *testing.T) {
	type requestBody struct {
		RequestID   string            `json:"request_id"`
		RecipientID string            `json:"recipient_id"`
		Kind        string            `json:"kind"`
		Data        map[string]string `json:"data"`
	}
	type receivedRequest struct {
		method      string
		path        string
		contentType string
		body        requestBody
	}
	received := make(chan receivedRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body requestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode push request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		received <- receivedRequest{
			method: r.Method, path: r.URL.Path,
			contentType: r.Header.Get("Content-Type"), body: body,
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(server.Close)
	gateway, err := New(server.URL, time.Second)
	require.NoError(t, err)
	t.Cleanup(gateway.Close)

	driverID, tripID := uuid.New(), uuid.New()
	require.NoError(t, gateway.RequestPosition(context.Background(), driverID, tripID))
	got := <-received
	require.Equal(t, http.MethodPost, got.method)
	require.Equal(t, "/api/v1/push", got.path)
	require.Equal(t, "application/json", got.contentType)
	require.Equal(t, driverID.String(), got.body.RecipientID)
	require.Equal(t, "REQUEST_POSITION", got.body.Kind)
	require.Equal(t, tripID.String(), got.body.Data["trip_id"])
	_, err = uuid.Parse(got.body.RequestID)
	require.NoError(t, err)
}

func TestGatewaySeparatesHTTPStatusAndTransportErrors(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			}))
			t.Cleanup(server.Close)
			gateway, err := New(server.URL, time.Second)
			require.NoError(t, err)
			t.Cleanup(gateway.Close)

			err = gateway.RequestPosition(context.Background(), uuid.New(), uuid.New())
			var statusErr *StatusError
			require.ErrorAs(t, err, &statusErr)
			require.Equal(t, status, statusErr.StatusCode)
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	gateway, err := New(server.URL, time.Second)
	require.NoError(t, err)
	t.Cleanup(gateway.Close)
	server.Close()

	err = gateway.RequestPosition(context.Background(), uuid.New(), uuid.New())
	var statusErr *StatusError
	require.Error(t, err)
	require.False(t, errors.As(err, &statusErr))
	require.ErrorContains(t, err, "push transport")
}
