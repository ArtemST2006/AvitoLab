package positions

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type positionRequester interface {
	RequestPosition(context.Context, uuid.UUID, uuid.UUID) error
}

type Pool struct {
	queue     *Queue
	requester positionRequester
	metrics   requestMetrics
	log       *slog.Logger
	workers   int
	timeout   time.Duration

	active atomic.Int64
	wg     sync.WaitGroup
	done   chan struct{}
}

func NewPool(queue *Queue, requester positionRequester, metrics requestMetrics, log *slog.Logger, workers int, timeout time.Duration) *Pool {
	return &Pool{
		queue:     queue,
		requester: requester,
		metrics:   metrics,
		log:       log,
		workers:   workers,
		timeout:   timeout,
		done:      make(chan struct{}),
	}
}

func (p *Pool) Start(ctx context.Context) {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.runWorker(ctx, i)
	}

	go func() {
		p.wg.Wait()
		close(p.done)
	}()
}

func (p *Pool) runWorker(ctx context.Context, number int) {
	defer p.wg.Done()

	p.log.InfoContext(ctx, "position worker started", slog.Int("worker", number))
	defer p.log.Info("position worker stopped", slog.Int("worker", number))

	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-p.queue.jobs:
			if !ok {
				return
			}
			p.process(ctx, task)
		}
	}
}

func (p *Pool) process(ctx context.Context, task queuedTrip) {
	defer p.queue.finish(task)

	if ctx.Err() != nil || !p.queue.isCurrent(task) {
		return
	}

	var options []trace.SpanStartOption
	if task.scanSpan.IsValid() {
		options = append(options, trace.WithLinks(trace.Link{SpanContext: task.scanSpan}))
	}
	ctx, span := otel.Tracer("tripgo/positions").Start(ctx, "positions.request", options...)
	defer span.End()

	p.active.Add(1)
	defer p.active.Add(-1)

	requestCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	started := time.Now()
	err := p.requester.RequestPosition(requestCtx, task.trip.DriverID, task.trip.TripID)
	result := "success"
	if err != nil {
		result = "error"
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		p.log.WarnContext(requestCtx, "request position failed",
			slog.String("trip_id", task.trip.TripID.String()),
			slog.Any("error", err))
	}
	p.metrics.Request(ctx, result, time.Since(started))
}

func (p *Pool) Wait(ctx context.Context) error {
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Pool) ActiveWorkers() int64 {
	return p.active.Load()
}
