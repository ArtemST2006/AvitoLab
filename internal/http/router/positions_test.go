package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ArtemST2006/AvitoLab/internal/domain/trip"
	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	positionlist "github.com/ArtemST2006/AvitoLab/internal/http/position/list"
	positionsave "github.com/ArtemST2006/AvitoLab/internal/http/position/save"
	"github.com/ArtemST2006/AvitoLab/internal/http/router"
	"github.com/ArtemST2006/AvitoLab/internal/model"
)

type saveStub struct {
	input  model.SavePositionInput
	result model.Position
	err    error
	calls  int
}

func (s *saveStub) Execute(_ context.Context, input model.SavePositionInput) (model.Position, error) {
	s.calls++
	s.input = input
	return s.result, s.err
}

type listStub struct {
	id     uuid.UUID
	result []model.Position
	err    error
	calls  int
}

func (s *listStub) Execute(_ context.Context, id uuid.UUID) ([]model.Position, error) {
	s.calls++
	s.id = id
	return s.result, s.err
}

type queueStub struct{ canceled []uuid.UUID }

func (q *queueStub) Cancel(id uuid.UUID) { q.canceled = append(q.canceled, id) }

type metricsStub struct{ saved int }

func (m *metricsStub) Saved(context.Context) { m.saved++ }

func positionRouter(t *testing.T, save *saveStub, list *listStub, queue *queueStub, metrics *metricsStub) http.Handler {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler, err := router.New(log, &router.Server{
		SavePosition:  positionsave.New(save, queue, metrics, log, time.Second),
		ListPositions: positionlist.New(list, log, time.Second),
	})
	require.NoError(t, err)
	return handler
}

func TestPostPositionHTTPContract(t *testing.T) {
	tripID := uuid.New()
	recordedAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	position := model.Position{
		ID: 42, TripID: tripID, Coordinates: model.Coordinates{Latitude: 59.935, Longitude: 30.335},
		RecordedAt: recordedAt,
	}
	validBody, err := json.Marshal(api.PositionData{
		Coordinates: api.Coordinates(position.Coordinates),
		RecordedAt:  recordedAt,
	})
	require.NoError(t, err)

	tests := []struct {
		name       string
		body       []byte
		saveErr    error
		wantStatus int
		wantCode   string
		wantCall   bool
	}{
		{name: "created", body: validBody, wantStatus: http.StatusCreated, wantCall: true},
		{name: "malformed JSON", body: []byte("{"), wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "invalid coordinates", body: []byte(`{"coordinates":{"latitude":91,"longitude":30},"recorded_at":"2026-08-24T12:00:05Z"}`), wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "trip not found", body: validBody, saveErr: trip.ErrTripNotFound, wantStatus: http.StatusNotFound, wantCode: "trip_not_found", wantCall: true},
		{name: "trip completed", body: validBody, saveErr: trip.ErrTripCompleted, wantStatus: http.StatusConflict, wantCode: "trip_completed", wantCall: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			save := &saveStub{result: position, err: tt.saveErr}
			queue := &queueStub{}
			metrics := &metricsStub{}
			handler := positionRouter(t, save, &listStub{}, queue, metrics)
			path := "/api/v1/trips/" + tripID.String() + "/positions"
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, bytes.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			require.Equal(t, tt.wantStatus, response.Code)
			if tt.wantCall {
				require.Equal(t, 1, save.calls)
				require.Equal(t, tripID, save.input.TripID)
				require.Equal(t, position.Coordinates, save.input.Coordinates)
				require.True(t, recordedAt.Equal(save.input.RecordedAt))
			} else {
				require.Zero(t, save.calls)
			}
			if tt.wantCode != "" {
				var problem api.Problem
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &problem))
				require.Equal(t, tt.wantCode, problem.Code)
				require.Empty(t, queue.canceled)
				require.Zero(t, metrics.saved)
				return
			}
			var got api.TripPosition
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &got))
			require.Equal(t, position.ID, got.Id)
			require.Equal(t, tripID, got.TripId)
			require.Equal(t, api.Coordinates(position.Coordinates), got.Coordinates)
			require.True(t, recordedAt.Equal(got.RecordedAt))
			require.Equal(t, []uuid.UUID{tripID}, queue.canceled)
			require.Equal(t, 1, metrics.saved)
		})
	}
}

func TestGetPositionsHTTPContract(t *testing.T) {
	tripID := uuid.New()
	recordedAt := time.Date(2026, time.August, 24, 12, 0, 5, 0, time.UTC)
	route := []model.Position{
		{ID: 1, TripID: tripID, Coordinates: model.Coordinates{Latitude: 59.93, Longitude: 30.33}, RecordedAt: recordedAt},
		{ID: 2, TripID: tripID, Coordinates: model.Coordinates{Latitude: 59.94, Longitude: 30.34}, RecordedAt: recordedAt.Add(time.Second)},
	}
	tests := []struct {
		name       string
		result     []model.Position
		listErr    error
		wantStatus int
		wantCode   string
	}{
		{name: "route", result: route, wantStatus: http.StatusOK},
		{name: "empty route", wantStatus: http.StatusOK},
		{name: "trip not found", listErr: trip.ErrTripNotFound, wantStatus: http.StatusNotFound, wantCode: "trip_not_found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := &listStub{result: tt.result, err: tt.listErr}
			handler := positionRouter(t, &saveStub{}, list, &queueStub{}, &metricsStub{})
			path := "/api/v1/trips/" + tripID.String() + "/positions"
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			require.Equal(t, tt.wantStatus, response.Code)
			require.Equal(t, 1, list.calls)
			require.Equal(t, tripID, list.id)
			if tt.wantCode != "" {
				var problem api.Problem
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &problem))
				require.Equal(t, tt.wantCode, problem.Code)
				return
			}
			var got api.TripPositionsResponse
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &got))
			require.Len(t, got.Positions, len(tt.result))
			for i, value := range tt.result {
				require.Equal(t, value.ID, got.Positions[i].Id)
				require.Equal(t, tripID, got.Positions[i].TripId)
				require.True(t, value.RecordedAt.Equal(got.Positions[i].RecordedAt))
			}
		})
	}
}
