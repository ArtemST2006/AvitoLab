// Package schemas содержит структуры данных хранилища, которых нет в OpenAPI-контракте.
package schemas

import (
	"time"

	"github.com/google/uuid"
)

// IdempotencyRecord — строка таблицы idempotency: ключ, хеш тела и созданная поездка.
type IdempotencyRecord struct {
	Key       uuid.UUID
	BodyHash  string
	TripID    uuid.UUID
	CreatedAt time.Time
}
