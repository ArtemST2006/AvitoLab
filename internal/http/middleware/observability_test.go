package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/ArtemST2006/AvitoLab/internal/http/middleware"
)

const panicRoute = "/panic"

func TestObservabilityMiddleware(t *testing.T) {
	previous := otel.GetTracerProvider()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() { otel.SetTracerProvider(previous); _ = provider.Shutdown(context.Background()) })
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = meterProvider.Shutdown(context.Background()) })

	var logs bytes.Buffer
	var panicSpan trace.SpanContext
	log := slog.New(panicTraceHandler{Handler: slog.NewJSONHandler(&logs, nil), span: &panicSpan})
	r := chi.NewRouter()
	metrics, err := middleware.NewMetrics(meterProvider.Meter("test"), func(method, path string) string {
		return r.Find(chi.NewRouteContext(), method, path)
	})
	require.NoError(t, err)
	r.Use(middleware.Recover(log), middleware.Tracing, middleware.Logging(log), metrics.Wrap)
	r.Get("/api/v1/trips/{tripId}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Post(panicRoute, func(_ http.ResponseWriter, _ *http.Request) { panic("secret panic") })

	id := uuid.NewString()
	for _, tc := range []struct {
		method, path, route string
		status              int
	}{
		{http.MethodGet, "/api/v1/trips/" + id, "/api/v1/trips/{tripId}", http.StatusOK},
		{http.MethodGet, "/missing/" + id, "unmatched", http.StatusNotFound},
		{http.MethodPost, panicRoute, panicRoute, http.StatusInternalServerError},
	} {
		response := httptest.NewRecorder()
		r.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))
		require.Equal(t, tc.status, response.Code)
		if tc.status == http.StatusInternalServerError {
			require.Contains(t, response.Body.String(), `"code":"internal_error"`)
			require.NotContains(t, response.Body.String(), "secret panic")
		}
	}

	var requestLogs []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		if entry["msg"] == "request handled" {
			requestLogs = append(requestLogs, entry)
		}
	}
	require.Len(t, requestLogs, 3)
	require.Equal(t, "/api/v1/trips/{tripId}", requestLogs[0]["route"])
	require.Equal(t, "unmatched", requestLogs[1]["route"])
	require.Equal(t, "INFO", requestLogs[1]["level"])
	require.Equal(t, "ERROR", requestLogs[2]["level"])
	require.Equal(t, float64(500), requestLogs[2]["status"])
	require.NotContains(t, logs.String(), id)

	spans := recorder.Ended()
	require.Len(t, spans, 3)
	require.Equal(t, "GET /api/v1/trips/{tripId}", spans[0].Name())
	require.Equal(t, "POST "+panicRoute, spans[2].Name())
	require.Equal(t, codes.Error, spans[2].Status().Code)
	require.True(t, panicSpan.IsValid())
	require.Equal(t, spans[2].SpanContext().TraceID(), panicSpan.TraceID())

	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &data))
	durations := metricByName(t, data, "http.server.request.duration").Data.(metricdata.Histogram[float64])
	require.Len(t, durations.DataPoints, 3)
	want := map[int]string{200: "/api/v1/trips/{tripId}", 404: "unmatched", 500: panicRoute}
	for _, point := range durations.DataPoints {
		require.EqualValues(t, 1, point.Count)
		status := int(attr(t, point.Attributes, "http.response.status_code").AsInt64())
		require.Equal(t, want[status], attr(t, point.Attributes, "http.route").AsString())
		delete(want, status)
	}
	require.Empty(t, want)
	active := metricByName(t, data, "http.server.active_requests").Data.(metricdata.Sum[int64])
	for _, point := range active.DataPoints {
		require.Zero(t, point.Value)
	}
}

type panicTraceHandler struct {
	slog.Handler
	span *trace.SpanContext
}

func (h panicTraceHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Message == "request panic" {
		*h.span = trace.SpanContextFromContext(ctx)
	}
	return h.Handler.Handle(ctx, record)
}

func TestActiveRequestsWhileHandlerRuns(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() { _ = provider.Shutdown(context.Background()) }()
	r := chi.NewRouter()
	metrics, err := middleware.NewMetrics(provider.Meter("test"), func(method, path string) string {
		return r.Find(chi.NewRouteContext(), method, path)
	})
	require.NoError(t, err)
	r.Use(metrics.Wrap)
	started := make(chan struct{})
	release := make(chan struct{})
	r.Get("/api/v1/trips/{tripId}", func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
	})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+uuid.NewString(), nil))
	}()
	<-started
	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &data))
	active := metricByName(t, data, "http.server.active_requests").Data.(metricdata.Sum[int64])
	require.Len(t, active.DataPoints, 1)
	require.EqualValues(t, 1, active.DataPoints[0].Value)
	require.Equal(t, attribute.StringValue("/api/v1/trips/{tripId}"), attr(t, active.DataPoints[0].Attributes, "http.route"))
	close(release)
	<-finished
}

func metricByName(t *testing.T, data metricdata.ResourceMetrics, name string) metricdata.Metrics {
	t.Helper()
	for _, scope := range data.ScopeMetrics {
		for _, measurement := range scope.Metrics {
			if measurement.Name == name {
				return measurement
			}
		}
	}
	t.Fatalf("metric %s missing", name)
	return metricdata.Metrics{}
}

func attr(t *testing.T, set attribute.Set, key string) attribute.Value {
	t.Helper()
	value, ok := set.Value(attribute.Key(key))
	require.True(t, ok, "attribute %s missing", key)
	return value
}
