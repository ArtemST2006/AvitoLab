package positions

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"github.com/ArtemST2006/AvitoLab/internal/model"
)

type queuedTrip struct {
	trip       model.StaleTrip
	generation uint64
	scanSpan   trace.SpanContext
}

type Queue struct {
	jobs       chan queuedTrip
	mu         sync.Mutex
	closed     bool
	pending    map[uuid.UUID]uint64
	generation uint64
}

func NewQueue(cap int) *Queue {
	return &Queue{
		jobs:    make(chan queuedTrip, cap),
		pending: make(map[uuid.UUID]uint64),
	}
}

func (q *Queue) TryEnqueue(ctx context.Context, trip model.StaleTrip) EnqueueResult {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return Dropped
	}
	if _, ok := q.pending[trip.TripID]; ok {
		return Duplicate
	}

	next := q.generation + 1
	task := queuedTrip{
		trip: trip, generation: next,
		scanSpan: trace.SpanContextFromContext(ctx),
	}

	select {
	case q.jobs <- task:
		q.generation = next
		q.pending[trip.TripID] = next
		return Queued
	default:
		return Dropped
	}
}

func (q *Queue) Cancel(tripID uuid.UUID) {
	q.mu.Lock()
	delete(q.pending, tripID)
	q.mu.Unlock()
}

func (q *Queue) isCurrent(task queuedTrip) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	generation, exists := q.pending[task.trip.TripID]
	return exists && generation == task.generation
}

func (q *Queue) finish(task queuedTrip) {
	q.mu.Lock()
	if q.pending[task.trip.TripID] == task.generation {
		delete(q.pending, task.trip.TripID)
	}
	q.mu.Unlock()
}

// Close вызывается после остановки сканера: новых задач больше не будет.
func (q *Queue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.closed {
		q.closed = true
		close(q.jobs)
	}
}

func (q *Queue) Size() int64 {
	return int64(len(q.jobs))
}
