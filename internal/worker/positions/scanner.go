package positions

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

// Ticker lets scanner tests drive ticks without waiting for wall-clock time.
type Ticker interface {
	Chan() <-chan time.Time
	Stop()
}

type ScannerOption func(*Scanner)

// WithTickerFactory replaces the clock used by Run. Configure it before calling Run.
func WithTickerFactory(factory func(time.Duration) Ticker) ScannerOption {
	return func(s *Scanner) { s.newTicker = factory }
}

type realTicker struct{ *time.Ticker }

func (t realTicker) Chan() <-chan time.Time { return t.C }

type Scanner struct {
	trips        staleTripRepository
	queue        enqueuer
	metrics      staleCheckMetrics
	requests     requestMetrics
	log          *slog.Logger
	interval     time.Duration
	staleAfter   time.Duration
	queryTimeout time.Duration
	newTicker    func(time.Duration) Ticker
}

func NewScanner(
	trips staleTripRepository,
	queue enqueuer,
	metrics staleCheckMetrics,
	requests requestMetrics,
	log *slog.Logger,
	interval, staleAfter, queryTimeout time.Duration,
	options ...ScannerOption,
) *Scanner {
	scanner := &Scanner{
		trips: trips, queue: queue, metrics: metrics, requests: requests, log: log,
		interval: interval, staleAfter: staleAfter, queryTimeout: queryTimeout,
		newTicker: func(interval time.Duration) Ticker { return realTicker{time.NewTicker(interval)} },
	}
	for _, option := range options {
		option(scanner)
	}
	return scanner
}

// Run занимает вызывающую горутину до отмены ctx.
func (s *Scanner) Run(ctx context.Context) {
	s.log.InfoContext(ctx, "position scanner started")
	defer s.log.Info("position scanner stopped")

	ticker := s.newTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.Chan():
			s.scan(ctx)
		}
	}
}

func (s *Scanner) scan(ctx context.Context) {
	ctx, span := otel.Tracer("tripgo/positions").Start(ctx, "positions.scan")
	defer span.End()

	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	trips, err := s.trips.ListStale(queryCtx, s.staleAfter)
	cancel()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if ctx.Err() == nil {
			s.log.ErrorContext(ctx, "list stale trips failed", slog.Any("error", err))
		}
		return
	}

	for _, trip := range trips {
		if ctx.Err() != nil {
			return
		}

		result := s.queue.TryEnqueue(ctx, trip)
		s.metrics.Record(ctx, string(result))
		if result == Dropped {
			s.requests.Dropped(ctx)
			s.log.WarnContext(ctx, "position request queue is full",
				slog.String("trip_id", trip.TripID.String()))
		}
	}
}
