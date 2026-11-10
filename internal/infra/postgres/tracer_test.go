package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestQueryTracerRecordsOperationCollectionAndError(t *testing.T) {
	previousTrace, previousMeter := otel.GetTracerProvider(), otel.GetMeterProvider()
	spans := tracetest.NewSpanRecorder()
	traceProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetTracerProvider(traceProvider)
	otel.SetMeterProvider(meterProvider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previousTrace)
		otel.SetMeterProvider(previousMeter)
		_ = traceProvider.Shutdown(context.Background())
		_ = meterProvider.Shutdown(context.Background())
	})

	tracer, err := newQueryTracer()
	require.NoError(t, err)
	parent, parentSpan := otel.Tracer("test").Start(context.Background(), "request")
	query := tracer.TraceQueryStart(parent, nil, pgx.TraceQueryStartData{SQL: "INSERT INTO trip_status_history (trip_id) VALUES ($1)"})
	tracer.TraceQueryEnd(query, nil, pgx.TraceQueryEndData{Err: errors.New("database failed")})
	parentSpan.End()

	ended := spans.Ended()
	require.Len(t, ended, 2)
	var found bool
	for _, span := range ended {
		if span.Name() == "request" {
			continue
		}
		found = true
		require.Equal(t, parentSpan.SpanContext().TraceID(), span.SpanContext().TraceID())
		require.Equal(t, sdktrace.Status{Code: codes.Error, Description: "database failed"}, span.Status())
		require.Equal(t, attribute.StringValue("INSERT"), spanAttribute(t, span.Attributes(), "db.operation.name"))
		require.Equal(t, attribute.StringValue("trip_status_history"), spanAttribute(t, span.Attributes(), "db.collection.name"))
		require.Equal(t, attribute.StringValue("*errors.errorString"), spanAttribute(t, span.Attributes(), "error.type"))
	}
	require.True(t, found)

	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &data))
	var histogram metricdata.Histogram[float64]
	for _, scope := range data.ScopeMetrics {
		for _, measurement := range scope.Metrics {
			if measurement.Name == "db.client.operation.duration" {
				histogram = measurement.Data.(metricdata.Histogram[float64])
			}
		}
	}
	require.Len(t, histogram.DataPoints, 1)
	require.EqualValues(t, 1, histogram.DataPoints[0].Count)
	require.Equal(t, attribute.StringValue("INSERT"), spanAttribute(t, histogram.DataPoints[0].Attributes.ToSlice(), "db.operation.name"))
	require.Equal(t, attribute.StringValue("trip_status_history"), spanAttribute(t, histogram.DataPoints[0].Attributes.ToSlice(), "db.collection.name"))
	require.Equal(t, attribute.StringValue("*errors.errorString"), spanAttribute(t, histogram.DataPoints[0].Attributes.ToSlice(), "error.type"))
}

func spanAttribute(t *testing.T, attrs []attribute.KeyValue, key string) attribute.Value {
	t.Helper()
	for _, attr := range attrs {
		if string(attr.Key) == key {
			return attr.Value
		}
	}
	t.Fatalf("attribute %s missing", key)
	return attribute.Value{}
}
