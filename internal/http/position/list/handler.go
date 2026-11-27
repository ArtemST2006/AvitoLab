package list

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/render"

	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/ArtemST2006/AvitoLab/internal/http/domainerror"
	httpposition "github.com/ArtemST2006/AvitoLab/internal/http/position"
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

	positions, err := h.usecase.Execute(ctx, id)
	if err != nil {
		status, body := domainerror.Problem(err)
		if status >= http.StatusInternalServerError {
			h.log.ErrorContext(ctx, "list trip positions failed", slog.Any("error", err))
		} else {
			h.log.InfoContext(ctx, "list trip positions failed", slog.Any("error", err))
		}
		domainerror.WriteProblem(w, r, status, body)
		return
	}

	response := api.TripPositionsResponse{Positions: make([]api.TripPosition, 0, len(positions))}
	for _, position := range positions {
		response.Positions = append(response.Positions, httpposition.ToAPI(position))
	}
	render.Status(r, http.StatusOK)
	render.JSON(w, r, response)
}
