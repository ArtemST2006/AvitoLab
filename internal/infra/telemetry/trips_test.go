package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type activeTripCounterStub struct{ calls int }

func (s *activeTripCounterStub) CountActive(context.Context) (int64, error) {
	s.calls++
	return 7, nil
}

func TestTripMetricsCountSuccessfulTransitions(t *testing.T) {
	previous := otel.GetMeterProvider()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		otel.SetMeterProvider(previous)
		_ = provider.Shutdown(context.Background())
	})

	metrics, err := NewTripMetrics()
	require.NoError(t, err)
	metrics.Created(context.Background())
	metrics.Completed(context.Background())

	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &data))
	values := make(map[string]int64)
	for _, scope := range data.ScopeMetrics {
		for _, measurement := range scope.Metrics {
			points := measurement.Data.(metricdata.Sum[int64]).DataPoints
			require.Len(t, points, 1)
			values[measurement.Name] = points[0].Value
		}
	}
	require.Equal(t, map[string]int64{
		"tripgo.trips.created":   1,
		"tripgo.trips.completed": 1,
	}, values)
}

func TestActiveTripsGaugeReadsCurrentCount(t *testing.T) {
	previous := otel.GetMeterProvider()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		otel.SetMeterProvider(previous)
		_ = provider.Shutdown(context.Background())
	})

	counter := new(activeTripCounterStub)
	registration, err := RegisterActiveTrips(counter, time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, registration.Unregister()) })

	for range 2 {
		var data metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(context.Background(), &data))
		var found bool
		for _, scope := range data.ScopeMetrics {
			for _, measurement := range scope.Metrics {
				if measurement.Name == "tripgo.trips.active" {
					found = true
					points := measurement.Data.(metricdata.Gauge[int64]).DataPoints
					require.Len(t, points, 1)
					require.EqualValues(t, 7, points[0].Value)
				}
			}
		}
		require.True(t, found)
	}
	require.Equal(t, 2, counter.calls)
}
