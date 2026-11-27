// Package create implements the create-trip use case.
package create

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ArtemST2006/AvitoLab/internal/domain/trip"
	"github.com/ArtemST2006/AvitoLab/internal/model"
)

const idempotencyTTL = time.Hour

// UseCase coordinates this trip operation.
type UseCase struct {
	trips   tripRepository
	keys    idempotencyRepository
	tx      txManager
	metrics tripMetrics
	now     func() time.Time
}

// New constructs the package component.
func New(trips tripRepository, keys idempotencyRepository, tx txManager, metrics ...tripMetrics) *UseCase {
	u := &UseCase{trips: trips, keys: keys, tx: tx, now: time.Now}
	if len(metrics) != 0 {
		u.metrics = metrics[0]
	}
	return u
}

// Execute runs this trip operation.
func (u *UseCase) Execute(ctx context.Context, input model.CreateTripInput) (model.CreateTripResult, error) {
	now := u.now().UTC()
	data := input.Data
	entity, err := trip.New(uuid.New(), data.UserID, data.DriverID,
		trip.Coordinates(data.StartPoint), trip.Coordinates(data.EndPoint), data.Price, now)
	if err != nil {
		return model.CreateTripResult{}, err
	}

	body, err := json.Marshal(data)
	if err != nil {
		return model.CreateTripResult{}, fmt.Errorf("encode trip for idempotency: %w", err)
	}
	sum := sha256.Sum256(body)
	bodyHash := hex.EncodeToString(sum[:])

	var result model.CreateTripResult
	err = u.tx.Do(ctx, func(ctx context.Context) error {
		if input.IdempotencyKey != nil {
			key := *input.IdempotencyKey
			expiresBefore := now.Add(-idempotencyTTL)
			record, findErr := u.keys.FindIdempotency(ctx, key, expiresBefore)
			if findErr == nil {
				return u.replay(ctx, record, bodyHash, &result)
			}
			if !errors.Is(findErr, trip.ErrIdempotencyNotFound) {
				return fmt.Errorf("find idempotency key: %w", findErr)
			}

			reserved, reserveErr := u.keys.ReserveIdempotency(ctx, key, bodyHash, expiresBefore)
			if reserveErr != nil {
				return fmt.Errorf("reserve idempotency key: %w", reserveErr)
			}
			if !reserved {
				record, findErr = u.keys.FindIdempotency(ctx, key, expiresBefore)
				if findErr != nil {
					return fmt.Errorf("read occupied idempotency key: %w", findErr)
				}
				return u.replay(ctx, record, bodyHash, &result)
			}
		}

		if insertErr := u.trips.Insert(ctx, entity); insertErr != nil {
			return insertErr
		}
		state := entity.Snapshot()
		if historyErr := u.trips.AppendStatus(ctx, state.ID, nil, state.Status, state.StartedAt); historyErr != nil {
			return historyErr
		}
		if input.IdempotencyKey != nil {
			if linkErr := u.keys.LinkIdempotency(ctx, *input.IdempotencyKey, state.ID); linkErr != nil {
				return linkErr
			}
		}
		result.Trip = model.FromDomain(entity)
		return nil
	})
	if err != nil {
		return model.CreateTripResult{}, err
	}
	if !result.Replayed && u.metrics != nil {
		u.metrics.Created(ctx)
	}
	return result, nil
}

func (u *UseCase) replay(ctx context.Context, record model.IdempotencyRecord, bodyHash string, result *model.CreateTripResult) error {
	if record.BodyHash != bodyHash {
		return trip.ErrIdempotencyConflict
	}
	entity, err := u.trips.Get(ctx, record.TripID)
	if err != nil {
		return fmt.Errorf("get idempotent trip: %w", err)
	}
	result.Trip = model.FromDomain(entity)
	result.Replayed = true
	return nil
}
