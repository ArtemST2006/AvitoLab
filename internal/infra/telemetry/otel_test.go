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
	collectormetric "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
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
	require.EqualValues(t, 1, metrics.ResourceMetrics[0].ScopeMetrics[0].Metrics[0].GetSum().DataPoints[0].GetAsInt())
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
