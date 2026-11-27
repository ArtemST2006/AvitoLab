package telemetry

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	collectormetric "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

func TestSetupExportsPendingTelemetry(t *testing.T) {
	var mu sync.Mutex
	traces := new(collectortrace.ExportTraceServiceRequest)
	metrics := new(collectormetric.ExportMetricsServiceRequest)
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		var message proto.Message
		switch r.URL.Path {
		case "/v1/traces":
			message = traces
		case "/v1/metrics":
			message = metrics
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := (proto.UnmarshalOptions{Merge: true}).Unmarshal(body, message); err != nil {
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer collector.Close()

	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", collector.URL)
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_PROTOCOL", "")
	for _, signal := range []string{"", "TRACES_", "METRICS_"} {
		t.Setenv("OTEL_EXPORTER_OTLP_"+signal+"COMPRESSION", "")
		t.Setenv("OTEL_EXPORTER_OTLP_"+signal+"HEADERS", "")
	}
	t.Setenv("OTEL_TRACES_SAMPLER", "always_on")
	t.Setenv("OTEL_SERVICE_NAME", "trip-test-service")
	previousTraces, previousMetrics, previousPropagator := otel.GetTracerProvider(), otel.GetMeterProvider(), otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(previousTraces)
		otel.SetMeterProvider(previousMetrics)
		otel.SetTextMapPropagator(previousPropagator)
	})

	logger, shutdown, err := Setup(context.Background(), "trip-service", "info", "test")
	require.NoError(t, err)
	require.NotNil(t, logger)
	ctx, span := otel.Tracer("test").Start(context.Background(), "pending span")
	counter, err := otel.Meter("test").Int64Counter("test.shutdown.counter")
	require.NoError(t, err)
	counter.Add(ctx, 1)
	for _, name := range []string{"http.server.request.duration", "db.client.operation.duration"} {
		histogram, histogramErr := otel.Meter("test").Float64Histogram(name, metric.WithUnit("s"))
		require.NoError(t, histogramErr)
		histogram.Record(ctx, 0.012)
	}
	span.End()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.NoError(t, shutdown(shutdownCtx))

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, traces.ResourceSpans)
	require.NotEmpty(t, metrics.ResourceMetrics)
	for _, resource := range []*resourcepb.Resource{traces.ResourceSpans[0].Resource, metrics.ResourceMetrics[0].Resource} {
		var name string
		for _, attr := range resource.Attributes {
			if attr.Key == "service.name" {
				name = attr.Value.GetStringValue()
			}
		}
		require.Equal(t, "trip-test-service", name)
	}
	require.Equal(t, "pending span", traces.ResourceSpans[0].ScopeSpans[0].Spans[0].Name)
	exported := make(map[string]*metricpb.Metric)
	for _, resourceMetrics := range metrics.ResourceMetrics {
		for _, scopeMetrics := range resourceMetrics.ScopeMetrics {
			for _, measurement := range scopeMetrics.Metrics {
				exported[measurement.Name] = measurement
			}
		}
	}
	require.Contains(t, exported, "test.shutdown.counter")
	require.EqualValues(t, 1, exported["test.shutdown.counter"].GetSum().DataPoints[0].GetAsInt())
	wantBounds := []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5}
	for _, name := range []string{"http.server.request.duration", "db.client.operation.duration"} {
		require.Contains(t, exported, name)
		require.Equal(t, "s", exported[name].Unit)
		points := exported[name].GetHistogram().DataPoints
		require.Len(t, points, 1)
		require.Equal(t, wantBounds, points[0].GetExplicitBounds())
		require.EqualValues(t, 1, points[0].GetCount())
	}
}

func TestSetupDoesNotRequireReachableCollector(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_PROTOCOL", "")
	previousTraces, previousMetrics, previousPropagator := otel.GetTracerProvider(), otel.GetMeterProvider(), otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(previousTraces)
		otel.SetMeterProvider(previousMetrics)
		otel.SetTextMapPropagator(previousPropagator)
	})

	_, shutdown, err := Setup(context.Background(), "trip-service", "info", "test")
	require.NoError(t, err)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = shutdown(shutdownCtx)
}
