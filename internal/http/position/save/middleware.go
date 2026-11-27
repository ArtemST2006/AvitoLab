package save

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/ArtemST2006/AvitoLab/internal/http/domainerror"
)

type bodyKey struct{}

// ValidateBody reads the position request once and restores it for OpenAPI validation.
func ValidateBody(log *slog.Logger) api.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/positions") {
				next.ServeHTTP(w, r)
				return
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				writeBadRequest(w, r, log, err)
				return
			}
			var data api.PositionData
			if err := json.Unmarshal(body, &data); err != nil {
				writeBadRequest(w, r, log, err)
				return
			}
			if data.RecordedAt.After(time.Now().Add(time.Minute)) {
				writeBadRequest(w, r, log, fmt.Errorf("recorded_at must not be more than one minute in the future"))
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			ctx := context.WithValue(r.Context(), bodyKey{}, data)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func validatedBody(r *http.Request) (api.PositionData, bool) {
	data, ok := r.Context().Value(bodyKey{}).(api.PositionData)
	return data, ok
}

func writeBadRequest(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	log.InfoContext(r.Context(), "invalid request body", slog.Any("error", err))
	status, body := domainerror.InvalidRequest(err.Error())
	domainerror.WriteProblem(w, r, status, body)
}
