// Package trip defines trip state and business rules.
package trip

import (
	"errors"
	"fmt"
)

var (
	// ErrTripNotFound identifies the corresponding trip failure.
	ErrTripNotFound = errors.New("trip not found")
	// ErrTripCompleted identifies the corresponding trip failure.
	ErrTripCompleted = errors.New("trip already completed")
	// ErrDriverBusy identifies the corresponding trip failure.
	ErrDriverBusy = errors.New("driver already has an active trip")
	// ErrIdempotencyConflict identifies the corresponding trip failure.
	ErrIdempotencyConflict = errors.New("idempotency key reused with different body")
	// ErrIdempotencyNotFound identifies the corresponding trip failure.
	ErrIdempotencyNotFound = errors.New("idempotency key not found")
)

// ValidationError describes a field that violates a trip rule.
type ValidationError struct {
	Field  string
	Reason string
}

// Error is part of this package's public contract.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Reason)
}
