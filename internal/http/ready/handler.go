// Package ready serves the readiness endpoint.
package ready

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/render"

	api "github.com/ArtemST2006/AvitoLab/internal/generated"
)

// Handler serves this endpoint.
type Handler struct {
	db      database
	log     *slog.Logger
	timeout time.Duration
}

// New constructs the package component.
func New(db database, log *slog.Logger, timeout time.Duration) *Handler {
	return &Handler{db: db, log: log, timeout: timeout}
}

// Handle processes an HTTP request.
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()
	if err := h.db.Ping(ctx); err != nil {
		h.log.ErrorContext(ctx, "database is not ready", slog.Any("error", err))
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, api.HealthResponse{Status: api.Unavailable})
		return
	}
	render.Status(r, http.StatusOK)
	render.JSON(w, r, api.HealthResponse{Status: api.Ok})
}
