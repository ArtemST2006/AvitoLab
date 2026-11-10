// Package finish implements the finish-trip use case.
package finish

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/ArtemST2006/AvitoLab/internal/domain/trip"
	"github.com/ArtemST2006/AvitoLab/internal/model"
)

// UseCase coordinates this trip operation.
type UseCase struct {
	trips   tripRepository
	tx      txManager
	metrics tripMetrics
	now     func() time.Time
}

// New constructs the package component.
func New(trips tripRepository, tx txManager, metrics ...tripMetrics) *UseCase {
	u := &UseCase{trips: trips, tx: tx, now: time.Now}
	if len(metrics) != 0 {
		u.metrics = metrics[0]
	}
	return u
}

// Execute runs this trip operation.
func (u *UseCase) Execute(ctx context.Context, id uuid.UUID) (model.Trip, error) {
	var result model.Trip
	err := u.tx.Do(ctx, func(ctx context.Context) error {
		entity, err := u.trips.GetForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if err := entity.Finish(u.now()); err != nil {
			return err
		}
		if err := u.trips.Update(ctx, entity); err != nil {
			return err
		}
		state := entity.Snapshot()
		from := trip.StatusActive
		if err := u.trips.AppendStatus(ctx, state.ID, &from, state.Status, *state.FinishedAt); err != nil {
			return err
		}
		result = model.FromDomain(entity)
		return nil
	})
	if err != nil {
		return model.Trip{}, err
	}
	if u.metrics != nil {
		u.metrics.Completed(ctx)
	}
	return result, nil
}
