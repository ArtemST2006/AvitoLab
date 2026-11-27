package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// TripMetrics counts committed trip state changes.
type TripMetrics struct {
	created   metric.Int64Counter
	completed metric.Int64Counter
}

func NewTripMetrics() (*TripMetrics, error) {
	meter := otel.Meter("tripgo/trips")
	created, err := meter.Int64Counter("tripgo.trips.created", metric.WithUnit("{trip}"))
	if err != nil {
		return nil, err
	}
	completed, err := meter.Int64Counter("tripgo.trips.completed", metric.WithUnit("{trip}"))
	if err != nil {
		return nil, err
	}
	return &TripMetrics{created: created, completed: completed}, nil
}

func (m *TripMetrics) Created(ctx context.Context)   { m.created.Add(ctx, 1) }
func (m *TripMetrics) Completed(ctx context.Context) { m.completed.Add(ctx, 1) }
