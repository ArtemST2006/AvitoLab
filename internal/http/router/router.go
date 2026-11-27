// Package router connects individual handlers to the generated API router.
package router

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	openapimw "github.com/oapi-codegen/nethttp-middleware"
	"go.opentelemetry.io/otel"

	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/ArtemST2006/AvitoLab/internal/http/domainerror"
	"github.com/ArtemST2006/AvitoLab/internal/http/health"
	"github.com/ArtemST2006/AvitoLab/internal/http/middleware"
	"github.com/ArtemST2006/AvitoLab/internal/http/ready"
	"github.com/ArtemST2006/AvitoLab/internal/http/trip/create"
	"github.com/ArtemST2006/AvitoLab/internal/http/trip/finish"
	"github.com/ArtemST2006/AvitoLab/internal/http/trip/get"
)

// Server adapts endpoint handlers to the generated API interface.
type Server struct {
	api.Unimplemented
	Create      *create.Handler
	Get         *get.Handler
	Finish      *finish.Handler
	HealthCheck *health.Handler
	ReadyCheck  *ready.Handler
}

var _ api.ServerInterface = (*Server)(nil)

// CreateTrip delegates to the create-trip handler.
func (s *Server) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	s.Create.Handle(w, r, params)
}

// GetTrip delegates to the get-trip handler.
func (s *Server) GetTrip(w http.ResponseWriter, r *http.Request, id api.TripId) {
	s.Get.Handle(w, r, id)
}

// FinishTrip delegates to the finish-trip handler.
func (s *Server) FinishTrip(w http.ResponseWriter, r *http.Request, id api.TripId) {
	s.Finish.Handle(w, r, id)
}

// Health delegates to the liveness handler.
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	s.HealthCheck.Handle(w, r)
}

// Ready delegates to the readiness handler.
func (s *Server) Ready(w http.ResponseWriter, r *http.Request) {
	s.ReadyCheck.Handle(w, r)
}

// New constructs the package component.
func New(log *slog.Logger, server *Server) (http.Handler, error) {
	spec, err := api.GetSpec()
	if err != nil {
		return nil, err
	}
	openAPI := openapimw.OapiRequestValidatorWithOptions(spec, &openapimw.Options{
		DoNotValidateServers: true,
		ErrorHandlerWithOpts: func(_ context.Context, err error, w http.ResponseWriter, r *http.Request, _ openapimw.ErrorHandlerOpts) {
			log.InfoContext(r.Context(), "invalid request", slog.Any("error", err))
			status, body := domainerror.InvalidRequest("invalid body")
			domainerror.WriteProblem(w, r, status, body)
		},
	})
	chiRouter := chi.NewRouter()
	metrics, err := middleware.NewMetrics(otel.Meter("tripgo/http"), func(method, path string) string {
		return chiRouter.Find(chi.NewRouteContext(), method, path)
	})
	if err != nil {
		return nil, err
	}
	chiRouter.Use(middleware.Recover(log), middleware.Tracing, middleware.Logging(log), metrics.Wrap, middleware.LimitBody)
	api.HandlerWithOptions(server, api.ChiServerOptions{
		BaseRouter: chiRouter,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			status, body := domainerror.InvalidRequest(err.Error())
			domainerror.WriteProblem(w, r, status, body)
		},
		Middlewares: []api.MiddlewareFunc{create.ValidateBody(log), openAPI},
	})
	return chiRouter, nil
}
