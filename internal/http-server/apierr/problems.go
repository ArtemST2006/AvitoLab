// Package apierr собирает ошибки HTTP API в формате RFC 9457 (application/problem+json).
package apierr

import (
	"encoding/json"
	"net/http"

	api "github.com/ArtemST2006/AvitoLab/internal/generated"
)

const typeBase = "https://tripgo.example/problems/"

// InvalidRequest — не прошла валидация тела или tripId не UUID.
func InvalidRequest(detail string) api.Problem {
	return newProblem(http.StatusBadRequest, "invalid_request", "invalid-request", "Invalid request", detail)
}

// TripNotFound — поездки с таким id нет.
func TripNotFound(detail string) api.Problem {
	return newProblem(http.StatusNotFound, "trip_not_found", "trip-not-found", "Trip not found", detail)
}

// Unavailable - один или несколько компонентов недоступны
func Unavailable(detail string) api.Problem {
	return newProblem(http.StatusServiceUnavailable, "service_unvaliable", "service-unvaliable", "Service unvaliable", detail)
}

// TripCompleted — попытка завершить уже завершенную поездку.
func TripCompleted(detail string) api.Problem {
	return newProblem(http.StatusConflict, "trip_completed", "trip-completed", "Trip completed", detail)
}

// DriverBusy — у водителя уже есть активная поездка.
func DriverBusy(detail string) api.Problem {
	return newProblem(http.StatusConflict, "driver_busy", "driver-busy", "Driver busy", detail)
}

// IdempotencyConflict — ключ идемпотентности уже использован с другим телом запроса.
func IdempotencyConflict(detail string) api.Problem {
	return newProblem(http.StatusConflict, "idempotency_conflict", "idempotency-conflict", "Idempotency conflict", detail)
}

// Internal — все остальные ошибки. Детали внутренней ошибки клиенту не отдаем.
func Internal() api.Problem {
	return newProblem(http.StatusInternalServerError, "internal_error", "internal-error", "Internal server error", "")
}

// Write отправляет problem клиенту. В instance кладет путь запроса.
func Write(w http.ResponseWriter, r *http.Request, p api.Problem) {
	instance := r.URL.Path
	p.Instance = &instance

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(int(p.Status))
	_ = json.NewEncoder(w).Encode(p)
}

func newProblem(status int32, code, slug, title, detail string) api.Problem {
	p := api.Problem{
		Type:   typeBase + slug,
		Title:  title,
		Status: status,
		Code:   code,
	}
	if detail != "" {
		p.Detail = &detail
	}

	return p
}
