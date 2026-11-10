package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func TestLoggerIncludesTraceContext(t *testing.T) {
	var output bytes.Buffer
	logger := newLogger("info", "dev", &output)
	span := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1},
		SpanID:     trace.SpanID{2},
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), span)

	logger.InfoContext(ctx, "request handled", slog.String("method", "POST"))
	var entry map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &entry))
	require.Equal(t, "INFO", entry["level"])
	require.Equal(t, "request handled", entry["msg"])
	require.Equal(t, "trip-service", entry["service"])
	require.Equal(t, "dev", entry["environment"])
	require.Equal(t, "POST", entry["method"])
	require.Equal(t, span.TraceID().String(), entry["trace_id"])
	require.Equal(t, span.SpanID().String(), entry["span_id"])
}

func TestLocalLoggerWritesJSONByDefault(t *testing.T) {
	t.Setenv("LOG_FORMAT", "")
	var output bytes.Buffer
	logger := newLogger("info", "local", &output)
	logger.Info("starting trip service", slog.String("address", ":8080"))

	var entry map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &entry))
	require.Equal(t, "trip-service", entry["service"])
	require.Equal(t, "local", entry["environment"])
	require.Equal(t, ":8080", entry["address"])
	require.Equal(t, "", entry["trace_id"])
	require.Equal(t, "", entry["span_id"])
}

func TestLocalLoggerUsesPrettyHandlerWhenRequested(t *testing.T) {
	t.Setenv("LOG_FORMAT", "pretty")
	var output bytes.Buffer
	logger := newLogger("info", "local", &output).With(slog.String("op", "test"))
	logger.Info("starting trip service", slog.String("address", ":8080"))

	line := output.String()
	require.Contains(t, line, "INFO:")
	require.Contains(t, line, "starting trip service")
	require.Contains(t, line, `"service": "trip-service"`)
	require.Contains(t, line, `"environment": "local"`)
	require.Contains(t, line, `"op": "test"`)
	require.Contains(t, line, `"address": ":8080"`)
	var entry map[string]any
	require.Error(t, json.Unmarshal(output.Bytes(), &entry))
}
