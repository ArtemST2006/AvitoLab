package positions

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ArtemST2006/AvitoLab/internal/model"
)

type staleRepositoryFunc func(context.Context, time.Duration) ([]model.StaleTrip, error)

func (f staleRepositoryFunc) ListStale(ctx context.Context, after time.Duration) ([]model.StaleTrip, error) {
	return f(ctx, after)
}

type manualTicker struct {
	ticks   chan time.Time
	stopped chan struct{}
	once    sync.Once
}

func newManualTicker() *manualTicker {
	return &manualTicker{
		ticks: make(chan time.Time, 1), stopped: make(chan struct{}),
	}
}

func (t *manualTicker) Chan() <-chan time.Time { return t.ticks }
func (t *manualTicker) Stop()                  { t.once.Do(func() { close(t.stopped) }) }

type checkMetricsRecorder struct{ results chan string }

func (m *checkMetricsRecorder) Record(_ context.Context, result string) {
	m.results <- result
}

type requestMetricsRecorder struct {
	results chan string
	dropped chan struct{}
}

func (m *requestMetricsRecorder) Dropped(context.Context) {
	m.dropped <- struct{}{}
}

func (m *requestMetricsRecorder) Request(_ context.Context, result string, _ time.Duration) {
	m.results <- result
}

func receiveTestValue[T any](t *testing.T, values <-chan T) T {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for background component")
		var zero T
		return zero
	}
}

func TestScannerRecordsQueuedDuplicateAndDropped(t *testing.T) {
	trips := []model.StaleTrip{
		{TripID: uuid.New(), DriverID: uuid.New()},
		{TripID: uuid.New(), DriverID: uuid.New()},
	}
	repo := staleRepositoryFunc(func(context.Context, time.Duration) ([]model.StaleTrip, error) {
		return trips, nil
	})
	queue := NewQueue(1)
	checks := &checkMetricsRecorder{results: make(chan string, 4)}
	requests := &requestMetricsRecorder{
		results: make(chan string, 1), dropped: make(chan struct{}, 2),
	}
	ticker := newManualTicker()
	scanner := NewScanner(repo, queue, checks, requests, slog.Default(),
		time.Hour, time.Minute, time.Second,
		WithTickerFactory(func(time.Duration) Ticker { return ticker }))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner.Run(ctx)
	}()
	t.Cleanup(func() { cancel(); receiveTestValue(t, done) })

	ticker.ticks <- time.Now()
	require.Equal(t, string(Queued), receiveTestValue(t, checks.results))
	require.Equal(t, string(Dropped), receiveTestValue(t, checks.results))
	receiveTestValue(t, requests.dropped)

	ticker.ticks <- time.Now()
	require.Equal(t, string(Duplicate), receiveTestValue(t, checks.results))
	require.Equal(t, string(Dropped), receiveTestValue(t, checks.results))
	receiveTestValue(t, requests.dropped)
	require.EqualValues(t, 1, queue.Size())

	cancel()
	receiveTestValue(t, done)
	receiveTestValue(t, ticker.stopped)
}

func TestScannerDoesNotOverlapTicks(t *testing.T) {
	entered := make(chan struct{}, 2)
	releaseFirst := make(chan struct{})
	var active, maximum, calls atomic.Int32
	repo := staleRepositoryFunc(func(ctx context.Context, _ time.Duration) ([]model.StaleTrip, error) {
		current := active.Add(1)
		if current > maximum.Load() {
			maximum.Store(current)
		}
		defer active.Add(-1)
		number := calls.Add(1)
		entered <- struct{}{}
		if number == 1 {
			select {
			case <-releaseFirst:
			case <-ctx.Done():
			}
		}
		return nil, nil
	})
	ticker := newManualTicker()
	scanner := NewScanner(repo, NewQueue(1),
		&checkMetricsRecorder{results: make(chan string, 1)},
		&requestMetricsRecorder{results: make(chan string, 1), dropped: make(chan struct{}, 1)},
		slog.Default(), time.Hour, time.Minute, time.Second,
		WithTickerFactory(func(time.Duration) Ticker { return ticker }))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner.Run(ctx)
	}()
	t.Cleanup(func() { cancel(); receiveTestValue(t, done) })

	ticker.ticks <- time.Now()
	receiveTestValue(t, entered)
	ticker.ticks <- time.Now()
	close(releaseFirst)
	receiveTestValue(t, entered)
	cancel()
	receiveTestValue(t, done)
	require.EqualValues(t, 1, maximum.Load())
}
