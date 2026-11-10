// Package finish serves the finish-trip endpoint.
package finish

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
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, id api.TripId) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()
	result, err := h.usecase.Execute(ctx, id)
	if err != nil {
		h.log.InfoContext(ctx, "finish trip failed", slog.Any("error", err))
		domainerror.Write(w, r, err)
		return
	}
	h.log.InfoContext(ctx, "trip completed", slog.String("trip_id", result.ID.String()))
	render.Status(r, http.StatusOK)
	render.JSON(w, r, httptrip.ToAPI(result))
}
