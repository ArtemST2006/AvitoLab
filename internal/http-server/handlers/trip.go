package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/ArtemST2006/AvitoLab/internal/http-server/apierr"
	"github.com/ArtemST2006/AvitoLab/internal/lib/logger/sl"
	"github.com/ArtemST2006/AvitoLab/internal/schemas"
	"github.com/ArtemST2006/AvitoLab/internal/storage/postgres"
	"github.com/go-chi/render"
)

// GetTrip возвращает поездку по её идентификатору.
//
// @openapi
// get /api/v1/trips/{tripId}
// operationId: getTrip
// tags: [Trips]
// responses:
//
//	200: Trip
//	400: BadRequest
//	404: TripNotFound
//	500: InternalError
func (s *Server) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	const op = "handlers.trip.GetTrip"

	log := s.log.With(
		slog.String("op", op),
	)

	ctx, cancel := context.WithTimeout(r.Context(), s.queryTimeout)
	defer cancel()

	trip, err := s.repo.GetTrip(ctx, tripID)
	if errors.Is(err, postgres.ErrNotFound) {
		log.Info("trip not found", sl.Err(err), slog.String("id", tripID.String()))

		problem := apierr.TripNotFound(fmt.Sprintf("trip with id: %s not found", tripID))
		apierr.Write(w, r, problem)

		return
	} else if err != nil {
		log.Info("internal error", sl.Err(err), slog.String("id", tripID.String()))

		problem := apierr.Internal()
		apierr.Write(w, r, problem)

		return
	}

	log.Info("got trip",
		slog.String("tripId", trip.Id.String()),
		slog.String("driverId", trip.DriverId.String()),
		slog.String("status", string(trip.Status)),
	)

	render.Status(r, http.StatusOK)
	render.JSON(w, r, trip)
}

// FinishTrip переводит активную поездку в статус completed.
//
// @openapi
// post /api/v1/trips/{tripId}/finish
// operationId: finishTrip
// tags: [Trips]
// responses:
//
//	200: Trip
//	400: BadRequest
//	404: TripNotFound
//	409: TripCompleted
//	500: InternalError
func (s *Server) FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	const op = "handlers.trip.FinishTrip"

	log := s.log.With(
		slog.String("op", op),
	)

	ctx, cancel := context.WithTimeout(r.Context(), s.queryTimeout)
	defer cancel()

	var trip api.Trip

	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		var errGetTrip error

		if trip, errGetTrip = s.repo.GetTrip(ctx, tripID); errGetTrip != nil {
			return errGetTrip
		}
		if stat := trip.Status; stat == api.Completed {
			return ErrAlreadyComplited
		}

		var errFinishTrip error

		trip, errFinishTrip = s.repo.FinishTrip(ctx, tripID)
		if errFinishTrip != nil {
			return errFinishTrip
		}

		from := api.Active
		return s.repo.AppendRecord(ctx, trip.Id, &from, api.Completed, *trip.FinishedAt)
	})

	if errors.Is(err, ErrAlreadyComplited) {
		log.Info("trip complited", sl.Err(err), slog.String("id", tripID.String()))

		problem := apierr.TripCompleted(fmt.Sprintf("trip already complited id: %s", tripID.String()))
		apierr.Write(w, r, problem)

		return
	}
	if errors.Is(err, postgres.ErrNotFound) {
		log.Info("trip not found", sl.Err(err), slog.String("id", tripID.String()))

		problem := apierr.TripNotFound(fmt.Sprintf("trip with id: %s not found", tripID.String()))
		apierr.Write(w, r, problem)

		return
	}
	if err != nil {
		log.Info("internal error", sl.Err(err), slog.String("id", tripID.String()))

		problem := apierr.Internal()
		apierr.Write(w, r, problem)

		return
	}

	log.Info("trip finished",
		slog.String("tripId", trip.Id.String()),
		slog.String("driverId", trip.DriverId.String()),
		slog.String("status", string(trip.Status)),
	)

	render.Status(r, http.StatusOK)
	render.JSON(w, r, trip)
}

// CreateTrip создаёт новую поездку.
//
// @openapi
// post /api/v1/trips
// operationId: createTrip
// tags: [Trips]
// params: IdempotencyKey
// body: TripData
// responses:
//
//	201: Trip
//	400: BadRequest
//	409: CreateTripConflict
//	500: InternalError
func (s *Server) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	const op = "hendlers.trip.CreateTrip"

	log := s.log.With(
		slog.String("op", op),
	)

	ctx, cancel := context.WithTimeout(r.Context(), s.queryTimeout)
	defer cancel()

	var body api.TripData
	if err := render.DecodeJSON(r.Body, &body); err != nil {
		log.Error("invalid request body", sl.Err(err))

		problem := apierr.InvalidRequest("bad request")
		apierr.Write(w, r, problem)

		return
	}
	if err := validateTripData(body); err != nil {
		log.Error("invalid request body", sl.Err(err))

		problem := apierr.InvalidRequest(fmt.Sprintf("bad request: %v", err))
		apierr.Write(w, r, problem)

		return
	}

	var (
		trip     api.Trip
		replayed bool // уже создали ранее
		bodyHash = hash(body)
	)

	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		if params.IdempotencyKey != nil {
			// идемпотентный сценарий
			// если есть ключь и хэш тела одинаковый -> Get и response 200
			// сли нет ключа, создаём запись в idempotency и вываливаемся из условия
			// если есть ключь но хэши разные -> response 409 conflict
			//
			// Ключ занимается первым: параллельный запрос с тем же ключом ждет нас на нем,
			// а не упирается в занятого водителя.
			var errIdem error
			trip, replayed, errIdem = s.reserveIdempotencyKey(ctx, *params.IdempotencyKey, bodyHash)
			if errIdem != nil || replayed {
				return errIdem
			}
		}

		// азовый сценарий

		var errCreate error
		trip, errCreate = s.repo.CreateTrip(ctx, body)
		if errCreate != nil {
			return errCreate
		}

		if err := s.repo.AppendRecord(ctx, trip.Id, nil, trip.Status, trip.StartedAt); err != nil {
			return err
		}

		if params.IdempotencyKey == nil {
			return nil
		}

		return s.repo.SetIdempotency(ctx, schemas.IdempotencyRecord{
			Key:      *params.IdempotencyKey,
			BodyHash: bodyHash,
			TripID:   trip.Id,
		})
	})

	if errors.Is(err, ErrIdempotencyConflict) {
		log.Info("idempotency conflict", sl.Err(err), slog.String("key", params.IdempotencyKey.String()))

		problem := apierr.IdempotencyConflict("Idempotency-Key was already used with a different request body")
		apierr.Write(w, r, problem)

		return
	}
	if errors.Is(err, postgres.ErrDriverBusy) {
		log.Info("driver busy", sl.Err(err), slog.String("driverId", body.DriverId.String()))

		problem := apierr.DriverBusy("driver busy")
		apierr.Write(w, r, problem)

		return
	}
	if err != nil {
		log.Info("internal error", sl.Err(err))

		problem := apierr.Internal()
		apierr.Write(w, r, problem)

		return
	}

	log.Info("trip created",
		slog.String("tripId", trip.Id.String()),
		slog.String("driverId", trip.DriverId.String()),
		slog.String("status", string(trip.Status)),
	)

	w.Header().Set("Location", "/api/v1/trips/"+trip.Id.String())
	if replayed {
		render.Status(r, http.StatusOK)
	} else {
		render.Status(r, http.StatusCreated)
	}
	render.JSON(w, r, trip)
}
