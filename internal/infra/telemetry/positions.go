package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// PositionMetrics records committed positions and Push Service attempts.
type PositionMetrics struct {
	saved    metric.Int64Counter
	requests metric.Int64Counter
	duration metric.Float64Histogram
}

func NewPositionMetrics() (*PositionMetrics, error) {
	meter := otel.Meter("tripgo/positions")
	saved, err := meter.Int64Counter("tripgo.positions.saved", metric.WithUnit("{position}"))
	if err != nil {
		return nil, err
	}
	requests, err := meter.Int64Counter("tripgo.position_requests", metric.WithUnit("{request}"))
	if err != nil {
		return nil, err
	}
	duration, err := meter.Float64Histogram("tripgo.position_requests.duration", metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	return &PositionMetrics{saved: saved, requests: requests, duration: duration}, nil
}

func (m *PositionMetrics) Saved(ctx context.Context) {
	m.saved.Add(ctx, 1)
}

func (m *PositionMetrics) Dropped(ctx context.Context) {
	m.requests.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "dropped")))
}

func (m *PositionMetrics) Request(ctx context.Context, result string, elapsed time.Duration) {
	attrs := metric.WithAttributes(attribute.String("result", result))
	m.requests.Add(ctx, 1, attrs)
	m.duration.Record(ctx, elapsed.Seconds(), attrs)
}

type positionQueueSize interface {
	Size() int64
}

type positionActiveWorkers interface {
	ActiveWorkers() int64
}

// RegisterPositionPoolGauges samples the queue and running workers at collection time.
func RegisterPositionPoolGauges(queue positionQueueSize, workers positionActiveWorkers) (metric.Registration, error) {
	meter := otel.Meter("tripgo/positions")
	queueSize, err := meter.Int64ObservableGauge("tripgo.position_requests.queue.size", metric.WithUnit("{task}"))
	if err != nil {
		return nil, err
	}
	workersActive, err := meter.Int64ObservableGauge("tripgo.position_requests.workers.active", metric.WithUnit("{worker}"))
	if err != nil {
		return nil, err
	}
	return meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		observer.ObserveInt64(queueSize, queue.Size())
		observer.ObserveInt64(workersActive, workers.ActiveWorkers())
		return nil
	}, queueSize, workersActive)
}
