// Package create serves the create-trip endpoint.
package create

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/render"

	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/ArtemST2006/AvitoLab/internal/http/domainerror"
	httptrip "github.com/ArtemST2006/AvitoLab/internal/http/trip"
)

// Handler serves this endpoint.
type Handler struct {
	usecase usecase
	log     *slog.Logger
	timeout time.Duration
}

// New constructs the package component.
func New(usecase usecase, log *slog.Logger, timeout time.Duration) *Handler {
	return &Handler{usecase: usecase, log: log, timeout: timeout}
}

// Handle processes an HTTP request.
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	body, ok := validatedBody(r)
	if !ok {
		h.log.ErrorContext(r.Context(), "validated trip body is missing")
		domainerror.Write(w, r, nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	result, err := h.usecase.Execute(ctx, toInput(body, params.IdempotencyKey))
	if err != nil {
		h.log.InfoContext(ctx, "create trip failed", slog.Any("error", err))
		domainerror.Write(w, r, err)
		return
	}
	h.log.InfoContext(ctx, "trip created", slog.String("tripId", result.Trip.ID.String()))
	w.Header().Set("Location", "/api/v1/trips/"+result.Trip.ID.String())
	if result.Replayed {
		render.Status(r, http.StatusOK)
	} else {
		render.Status(r, http.StatusCreated)
	}
	render.JSON(w, r, httptrip.ToAPI(result.Trip))
}
