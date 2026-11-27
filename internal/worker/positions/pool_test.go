package positions

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ArtemST2006/AvitoLab/internal/model"
)

type requesterFunc func(context.Context, uuid.UUID, uuid.UUID) error

func (f requesterFunc) RequestPosition(ctx context.Context, driverID, tripID uuid.UUID) error {
	return f(ctx, driverID, tripID)
}

func waitForPool(t *testing.T, pool *Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, pool.Wait(ctx))
}

func TestPoolUsesFixedNumberOfWorkers(t *testing.T) {
	queue := NewQueue(3)
	for range 3 {
		require.Equal(t, Queued, queue.TryEnqueue(context.Background(),
			model.StaleTrip{TripID: uuid.New(), DriverID: uuid.New()}))
	}
	queue.Close()

	started := make(chan uuid.UUID, 3)
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	requester := requesterFunc(func(ctx context.Context, _, tripID uuid.UUID) error {
		started <- tripID
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	metrics := &requestMetricsRecorder{results: make(chan string, 3), dropped: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	pool := NewPool(queue, requester, metrics, slog.Default(), 2, 5*time.Second)
	pool.Start(ctx)

	receiveTestValue(t, started)
	receiveTestValue(t, started)
	require.EqualValues(t, 2, pool.ActiveWorkers())
	releaseOnce.Do(func() { close(release) })
	receiveTestValue(t, started)
	for range 3 {
		require.Equal(t, "success", receiveTestValue(t, metrics.results))
	}
	waitForPool(t, pool)
	require.Zero(t, pool.ActiveWorkers())
}

func TestPoolSkipsCancelledQueuedTrip(t *testing.T) {
	queue := NewQueue(1)
	trip := model.StaleTrip{TripID: uuid.New(), DriverID: uuid.New()}
	require.Equal(t, Queued, queue.TryEnqueue(context.Background(), trip))
	queue.Cancel(trip.TripID)
	queue.Close()

	var calls atomic.Int32
	requester := requesterFunc(func(context.Context, uuid.UUID, uuid.UUID) error {
		calls.Add(1)
		return nil
	})
	metrics := &requestMetricsRecorder{results: make(chan string, 1), dropped: make(chan struct{}, 1)}
	pool := NewPool(queue, requester, metrics, slog.Default(), 1, time.Second)
	pool.Start(context.Background())
	waitForPool(t, pool)
	require.Zero(t, calls.Load())
}

func TestPoolContinuesAfterPushError(t *testing.T) {
	queue := NewQueue(2)
	for range 2 {
		require.Equal(t, Queued, queue.TryEnqueue(context.Background(),
			model.StaleTrip{TripID: uuid.New(), DriverID: uuid.New()}))
	}
	queue.Close()

	var calls atomic.Int32
	requester := requesterFunc(func(context.Context, uuid.UUID, uuid.UUID) error {
		if calls.Add(1) == 1 {
			return errors.New("push unavailable")
		}
		return nil
	})
	metrics := &requestMetricsRecorder{results: make(chan string, 2), dropped: make(chan struct{}, 1)}
	pool := NewPool(queue, requester, metrics, slog.Default(), 1, time.Second)
	pool.Start(context.Background())
	waitForPool(t, pool)
	require.EqualValues(t, 2, calls.Load())
	require.Equal(t, "error", receiveTestValue(t, metrics.results))
	require.Equal(t, "success", receiveTestValue(t, metrics.results))
}

func TestPoolExitsWhenContextIsCancelled(t *testing.T) {
	queue := NewQueue(1)
	metrics := &requestMetricsRecorder{results: make(chan string, 1), dropped: make(chan struct{}, 1)}
	var calls atomic.Int32
	pool := NewPool(queue, requesterFunc(func(context.Context, uuid.UUID, uuid.UUID) error {
		calls.Add(1)
		return nil
	}), metrics, slog.Default(), 2, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	pool.Start(ctx)
	cancel()
	waitForPool(t, pool)
	require.Zero(t, calls.Load())
	queue.Close()
}

func TestPoolAppliesRequestTimeout(t *testing.T) {
	queue := NewQueue(1)
	require.Equal(t, Queued, queue.TryEnqueue(context.Background(),
		model.StaleTrip{TripID: uuid.New(), DriverID: uuid.New()}))
	queue.Close()

	errorsSeen := make(chan error, 1)
	requester := requesterFunc(func(ctx context.Context, _, _ uuid.UUID) error {
		<-ctx.Done()
		errorsSeen <- ctx.Err()
		return ctx.Err()
	})
	metrics := &requestMetricsRecorder{results: make(chan string, 1), dropped: make(chan struct{}, 1)}
	pool := NewPool(queue, requester, metrics, slog.Default(), 1, 20*time.Millisecond)
	pool.Start(context.Background())
	waitForPool(t, pool)
	require.ErrorIs(t, receiveTestValue(t, errorsSeen), context.DeadlineExceeded)
	require.Equal(t, "error", receiveTestValue(t, metrics.results))
}
