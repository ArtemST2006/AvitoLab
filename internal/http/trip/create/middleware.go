// Package create serves the create-trip endpoint.
package create

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/ArtemST2006/AvitoLab/internal/http/domainerror"
)

type bodyKey struct{}

// ValidateBody reads the create request once and restores it for OpenAPI validation.
func ValidateBody(log *slog.Logger) api.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/api/v1/trips" {
				next.ServeHTTP(w, r)
				return
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				writeBadRequest(w, r, log, err)
				return
			}
			var data api.TripData
			if err := json.Unmarshal(body, &data); err != nil {
				writeBadRequest(w, r, log, err)
				return
			}
			if data.UserId == uuid.Nil {
				writeBadRequest(w, r, log, fmt.Errorf("user_id must not be nil UUID"))
				return
			}
			if data.DriverId == uuid.Nil {
				writeBadRequest(w, r, log, fmt.Errorf("driver_id must not be nil UUID"))
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			ctx := context.WithValue(r.Context(), bodyKey{}, data)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func validatedBody(r *http.Request) (api.TripData, bool) {
	data, ok := r.Context().Value(bodyKey{}).(api.TripData)
	return data, ok
}

func writeBadRequest(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	log.InfoContext(r.Context(), "invalid request body", slog.Any("error", err))
	status, body := domainerror.InvalidRequest(err.Error())
	domainerror.WriteProblem(w, r, status, body)
}
