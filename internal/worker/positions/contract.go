package positions

import (
	"context"
	"time"

	"github.com/ArtemST2006/AvitoLab/internal/model"
)

type EnqueueResult string

const (
	Queued    EnqueueResult = "queued"
	Duplicate EnqueueResult = "duplicate"
	Dropped   EnqueueResult = "dropped"
)

type staleTripRepository interface {
	ListStale(context.Context, time.Duration) ([]model.StaleTrip, error)
}

// TryEnqueue обязан вернуть результат сразу, без ожидания свободного места.
type enqueuer interface {
	TryEnqueue(context.Context, model.StaleTrip) EnqueueResult
}

type staleCheckMetrics interface {
	Record(context.Context, string)
}

type requestMetrics interface {
	Dropped(context.Context)
	Request(context.Context, string, time.Duration)
}
