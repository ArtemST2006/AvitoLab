package handlers

import (
	"context"
	"log/slog"
	"net/http"

	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/ArtemST2006/AvitoLab/internal/lib/logger/sl"
	"github.com/go-chi/render"
)

// Health отвечает, что процесс жив (GET /health).
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	render.Status(r, http.StatusOK)
	render.JSON(w, r, api.HealthResponse{
		Status: api.Ok,
	})
}

// Ready проверяет доступность компонентов: бд (GET /ready).
func (s *Server) Ready(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.check.Ready"

	log := s.log.With(
		slog.String("op", op),
	)

	ctx, cancel := context.WithTimeout(r.Context(), s.queryTimeout)
	defer cancel()

	if err := s.repo.Ping(ctx); err != nil {
		log.Error("failed db ping", sl.Err(err))

		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, api.HealthResponse{
			Status: api.Unavailable,
		})

		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, api.HealthResponse{
		Status: api.Ok,
	})
}
