package model

import (
	"time"

	"github.com/google/uuid"
)

type SavePositionInput struct {
	TripID      uuid.UUID
	Coordinates Coordinates
	RecordedAt  time.Time
}

type Position struct {
	ID          int64
	TripID      uuid.UUID
	Coordinates Coordinates
	RecordedAt  time.Time
}
