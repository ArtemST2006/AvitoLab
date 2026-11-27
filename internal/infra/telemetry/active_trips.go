package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type activeTripCounter interface {
	CountActive(context.Context) (int64, error)
}

// RegisterActiveTrips measures the current database state at collection time.
func RegisterActiveTrips(counter activeTripCounter, timeout time.Duration) (metric.Registration, error) {
	meter := otel.Meter("tripgo/trips")
	gauge, err := meter.Int64ObservableGauge("tripgo.trips.active", metric.WithUnit("{trip}"))
	if err != nil {
		return nil, err
	}
	return meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		queryCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		count, err := counter.CountActive(queryCtx)
		if err != nil {
			return err
		}
		observer.ObserveInt64(gauge, count)
		return nil
	}, gauge)
}
