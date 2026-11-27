package save

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

type Handler struct {
	usecase usecase
	metrics positionMetrics
	log     *slog.Logger
	timeout time.Duration
	queue   positionQueue
}

// New constructs the package component.
func New(usecase usecase, queue positionQueue, metrics positionMetrics, log *slog.Logger, timeout time.Duration) *Handler {
	return &Handler{usecase: usecase, metrics: metrics, log: log, timeout: timeout, queue: queue}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, id api.TripId) {
	body, ok := validatedBody(r)
	if !ok {
		h.log.ErrorContext(r.Context(), "validated position body is missing")
		domainerror.Write(w, r, nil)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	result, err := h.usecase.Execute(ctx, toInput(body, id))
	if err != nil {
		status, problem := domainerror.Problem(err)
		if status >= http.StatusInternalServerError {
			h.log.ErrorContext(ctx, "save position failed", slog.Any("error", err))
		} else {
			h.log.InfoContext(ctx, "save position failed", slog.Any("error", err))
		}
		domainerror.WriteProblem(w, r, status, problem)
		return
	}

	h.queue.Cancel(result.TripID)
	h.metrics.Saved(ctx)

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, httpposition.ToAPI(result))
}
