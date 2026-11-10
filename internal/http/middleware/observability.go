package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"

	"github.com/ArtemST2006/AvitoLab/internal/http/domainerror"
)

const unmatchedRoute = "unmatched"

type requestTraceKey struct{}

type requestTrace struct {
	span trace.SpanContext
}

// Recover converts a handler panic to a public problem response.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			state := &requestTrace{}
			r = r.WithContext(context.WithValue(r.Context(), requestTraceKey{}, state))
			defer func() {
				if value := recover(); value != nil {
					logCtx := r.Context()
					if state.span.IsValid() {
						logCtx = trace.ContextWithSpanContext(logCtx, state.span)
					}
					log.ErrorContext(logCtx, "request panic", slog.Any("panic", value), slog.String("stack", string(debug.Stack())))
					status, body := domainerror.Problem(nil)
					domainerror.WriteProblem(w, r, status, body)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Tracing starts a server span and names it after the matched route.
func Tracing(next http.Handler) http.Handler {
	// Request metrics are recorded by Metrics so every series has the chi route.
	traced := otelhttp.NewMiddleware("HTTP", otelhttp.WithMeterProvider(metricnoop.NewMeterProvider()))
	return traced(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if state, ok := r.Context().Value(requestTraceKey{}).(*requestTrace); ok {
			state.span = trace.SpanContextFromContext(r.Context())
		}
		wrapped := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
		defer func() {
			span := trace.SpanFromContext(r.Context())
			route := routePattern(r)
			r.Pattern = route
			span.SetName(r.Method + " " + route)
			status := wrapped.Status()
			if value := recover(); value != nil {
				status = http.StatusInternalServerError
				span.RecordError(errors.New("handler panic"))
				span.SetStatus(codes.Error, "panic")
				span.SetAttributes(attribute.String("error.type", "panic"))
				panic(value)
			}
			if status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(status))
				span.SetAttributes(attribute.String("error.type", http.StatusText(status)))
			}
		}()
		next.ServeHTTP(wrapped, r)
	}))
}

// Logging writes exactly one completion record for each request.
func Logging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			wrapped := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			defer func() {
				status := wrapped.Status()
				if value := recover(); value != nil {
					status = http.StatusInternalServerError
					writeRequestLog(log, r, status, time.Since(started))
					panic(value)
				}
				if status == 0 {
					status = http.StatusOK
				}
				writeRequestLog(log, r, status, time.Since(started))
			}()
			next.ServeHTTP(wrapped, r)
		})
	}
}

func writeRequestLog(log *slog.Logger, r *http.Request, status int, duration time.Duration) {
	attrs := []any{
		slog.String("method", r.Method),
		slog.String("route", routePattern(r)),
		slog.Int("status", status),
		slog.Int64("duration_ms", duration.Milliseconds()),
	}
	if status >= http.StatusInternalServerError {
		log.ErrorContext(r.Context(), "request handled", attrs...)
	} else {
		log.InfoContext(r.Context(), "request handled", attrs...)
	}
}

func routePattern(r *http.Request) string {
	if pattern := chi.RouteContext(r.Context()).RoutePattern(); pattern != "" {
		return pattern
	}
	return unmatchedRoute
}

// Metrics records request duration and the number of requests currently running.
type Metrics struct {
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
	match    func(method, path string) string
}

func NewMetrics(meter metric.Meter, match func(method, path string) string) (*Metrics, error) {
	duration, err := meter.Float64Histogram("http.server.request.duration", metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	active, err := meter.Int64UpDownCounter("http.server.active_requests", metric.WithUnit("{request}"))
	if err != nil {
		return nil, err
	}
	return &Metrics{duration: duration, active: active, match: match}, nil
}

func (m *Metrics) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		route := m.match(r.Method, r.URL.Path)
		if route == "" {
			route = unmatchedRoute
		}
		base := []attribute.KeyValue{attribute.String("http.request.method", r.Method), attribute.String("http.route", route)}
		m.active.Add(r.Context(), 1, metric.WithAttributes(base...))
		wrapped := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
		defer func() {
			status := wrapped.Status()
			if value := recover(); value != nil {
				status = http.StatusInternalServerError
				m.record(r, started, base, status)
				panic(value)
			}
			if status == 0 {
				status = http.StatusOK
			}
			m.record(r, started, base, status)
		}()
		next.ServeHTTP(wrapped, r)
	})
}

func (m *Metrics) record(r *http.Request, started time.Time, base []attribute.KeyValue, status int) {
	m.duration.Record(r.Context(), time.Since(started).Seconds(), metric.WithAttributes(
		append(base, attribute.Int("http.response.status_code", status))...,
	))
	m.active.Add(r.Context(), -1, metric.WithAttributes(base...))
}
