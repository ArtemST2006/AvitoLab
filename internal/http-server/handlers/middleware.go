package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/ArtemST2006/AvitoLab/internal/http-server/apierr"
	"github.com/ArtemST2006/AvitoLab/internal/lib/logger/sl"
	"github.com/google/uuid"
)

const maxRequestBodyBytes = 1 << 20

type tripBodyKey struct{}

// LimitBody ограничивает тело запроса до 1 МиБ до запуска OpenAPI-валидатора.
func LimitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		next.ServeHTTP(w, r)
	})
}

// ValidateTripBody разбирает всё тело поездки и кладёт данные в контекст.
func ValidateTripBody(log *slog.Logger) api.MiddlewareFunc {
	log = log.With(slog.String("op", "handlers.ValidateTripBody"))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/api/v1/trips" {
				next.ServeHTTP(w, r)
				return
			}

			badRequest := func(err error) {
				log.InfoContext(r.Context(), "invalid request body", sl.Err(err))
				apierr.Write(w, r, apierr.InvalidRequest(err.Error()))
			}

			data, err := io.ReadAll(r.Body)
			if err != nil {
				badRequest(err)
				return
			}

			var trip api.TripData
			if err := json.Unmarshal(data, &trip); err != nil {
				badRequest(err)
				return
			}

			if trip.UserId == uuid.Nil {
				badRequest(errors.New("user_id must not be nil UUID"))
				return
			}
			if trip.DriverId == uuid.Nil {
				badRequest(errors.New("driver_id must not be nil UUID"))
				return
			}

			ctx := context.WithValue(r.Context(), tripBodyKey{}, trip)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
