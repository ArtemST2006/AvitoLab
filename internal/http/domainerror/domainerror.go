// Package domainerror maps application errors to HTTP problems.
package domainerror

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ArtemST2006/AvitoLab/internal/domain/trip"
	api "github.com/ArtemST2006/AvitoLab/internal/generated"
)

const typeBase = "https://tripgo.example/problems/"

// Problem maps an application error to the public HTTP problem contract.
func Problem(err error) (int, api.Problem) {
	var validation *trip.ValidationError
	switch {
	case errors.As(err, &validation):
		return problem(http.StatusBadRequest, "invalid_request", "invalid-request", "Invalid request", validation.Error())
	case errors.Is(err, trip.ErrTripNotFound):
		return problem(http.StatusNotFound, "trip_not_found", "trip-not-found", "Trip not found", err.Error())
	case errors.Is(err, trip.ErrTripCompleted):
		return problem(http.StatusConflict, "trip_completed", "trip-completed", "Trip completed", err.Error())
	case errors.Is(err, trip.ErrDriverBusy):
		return problem(http.StatusConflict, "driver_busy", "driver-busy", "Driver busy", err.Error())
	case errors.Is(err, trip.ErrIdempotencyConflict):
		return problem(http.StatusConflict, "idempotency_conflict", "idempotency-conflict", "Idempotency conflict", err.Error())
	default:
		return problem(http.StatusInternalServerError, "internal_error", "internal-error", "Internal server error", "")
	}
}

// InvalidRequest builds a bad-request problem.
func InvalidRequest(detail string) (int, api.Problem) {
	return problem(http.StatusBadRequest, "invalid_request", "invalid-request", "Invalid request", detail)
}

// Write maps and sends an application error.
func Write(w http.ResponseWriter, r *http.Request, err error) {
	status, body := Problem(err)
	WriteProblem(w, r, status, body)
}

// WriteProblem sends a problem response.
func WriteProblem(w http.ResponseWriter, r *http.Request, status int, body api.Problem) {
	instance := r.URL.Path
	body.Instance = &instance
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func problem(status int32, code, slug, title, detail string) (int, api.Problem) {
	body := api.Problem{
		Type: typeBase + slug, Title: title, Status: status, Code: code,
	}
	if detail != "" {
		body.Detail = &detail
	}
	return int(status), body
}
