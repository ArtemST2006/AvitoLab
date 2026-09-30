package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/ArtemST2006/AvitoLab/internal/schemas"
	"github.com/ArtemST2006/AvitoLab/internal/storage/postgres"
)

// ErrIdempotencyConflict — ключ идемпотентности уже использован с другим телом.
var ErrIdempotencyConflict = errors.New("idempotency key reused with different body")

func hash(body api.TripData) string {
	b, _ := json.Marshal(body)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// reserveIdempotencyKey вызывается внутри Do. Нужен для вынесения логики обращений к бд
func (s *Server) reserveIdempotencyKey(ctx context.Context, key uuid.UUID, bodyHash string) (api.Trip, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	record, err := s.repo.GetIdempotency(ctx, key.String())
	if errors.Is(err, postgres.ErrNotFound) {
		// сли нет ключа, создаём запись в idempotency и вываливаемся из условия
		err = s.repo.SetIdempotency(ctx, schemas.IdempotencyRecord{Key: key, BodyHash: bodyHash})
		if err == nil {
			return api.Trip{}, false, nil
		}
		if !errors.Is(err, postgres.ErrIdempotencyExists) {
			return api.Trip{}, false, err
		}

		record, err = s.repo.GetIdempotency(ctx, key.String())
	}
	if err != nil {
		return api.Trip{}, false, fmt.Errorf("get idempotency: %w", err)
	}

	if record.BodyHash != bodyHash {
		// если есть ключь но хэши разные -> response 409 conflict
		return api.Trip{}, false, ErrIdempotencyConflict
	}

	// если есть ключь и хэш тела одинаковый -> Get и response 200
	trip, err := s.repo.GetTrip(ctx, record.TripID)
	if err != nil {
		return api.Trip{}, false, fmt.Errorf("get idempotent trip: %w", err)
	}

	return trip, true, nil
}
