package positions

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ArtemST2006/AvitoLab/internal/model"
)

func TestQueueDeduplicatesDropsAndCancels(t *testing.T) {
	queue := NewQueue(1)
	first := model.StaleTrip{TripID: uuid.New(), DriverID: uuid.New()}
	second := model.StaleTrip{TripID: uuid.New(), DriverID: uuid.New()}

	require.Equal(t, Queued, queue.TryEnqueue(context.Background(), first))
	require.Equal(t, Duplicate, queue.TryEnqueue(context.Background(), first))
	require.Equal(t, Dropped, queue.TryEnqueue(context.Background(), second))
	require.EqualValues(t, 1, queue.Size())

	queue.Cancel(first.TripID)
	oldTask := <-queue.jobs
	require.False(t, queue.isCurrent(oldTask))

	require.Equal(t, Queued, queue.TryEnqueue(context.Background(), first))
	newTask := <-queue.jobs
	require.True(t, queue.isCurrent(newTask))
	queue.finish(oldTask)
	require.True(t, queue.isCurrent(newTask), "old worker must not clear a newer task")
	queue.finish(newTask)

	queue.Close()
	require.Equal(t, Dropped, queue.TryEnqueue(context.Background(), first))
}
