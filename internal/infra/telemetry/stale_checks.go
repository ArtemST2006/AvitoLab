package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type StaleCheckMetrics struct {
	checks metric.Int64Counter
}

func NewStaleCheckMetrics() (*StaleCheckMetrics, error) {
	checks, err := otel.Meter("tripgo/positions").Int64Counter(
		"tripgo.stale_checks",
		metric.WithUnit("{check}"),
	)
	if err != nil {
		return nil, err
	}
	return &StaleCheckMetrics{checks: checks}, nil
}

func (m *StaleCheckMetrics) Record(ctx context.Context, result string) {
	m.checks.Add(ctx, 1,
		metric.WithAttributes(attribute.String("result", result)))
}
