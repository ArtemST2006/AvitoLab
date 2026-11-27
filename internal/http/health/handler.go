// Package health serves the liveness endpoint.
package health

import (
	"net/http"

	"github.com/go-chi/render"

	api "github.com/ArtemST2006/AvitoLab/internal/generated"
)

// Handler serves this endpoint.
type Handler struct{}

// New constructs the package component.
func New() *Handler {
	return &Handler{}
}

// Handle processes an HTTP request.
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	render.Status(r, http.StatusOK)
	render.JSON(w, r, api.HealthResponse{Status: api.Ok})
}
