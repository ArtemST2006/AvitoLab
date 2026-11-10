// Package telemetry configures OpenTelemetry once for the service process.
package telemetry

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Setup installs providers for traces and metrics and returns the service logger.
// OTEL_SERVICE_NAME can override defaultName. Call shutdown after HTTP handlers drain.
func Setup(ctx context.Context, defaultName, level, environment string) (_ *slog.Logger, shutdown func(context.Context) error, err error) {
	logger := NewLogger(level, environment)
	otel.SetErrorHandler(slogErrorHandler{log: logger})
	var closers []func(context.Context) error
	shutdown = func(ctx context.Context) error {
		var result error
		for i := len(closers) - 1; i >= 0; i-- {
			result = errors.Join(result, closers[i](ctx))
		}
		return result
	}
	defer func() {
		if err != nil {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			err = errors.Join(err, shutdown(cleanupCtx))
		}
	}()
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return nil, shutdown, errors.New("set OTEL_EXPORTER_OTLP_ENDPOINT before starting the service")
	}
	if protocol := os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"); protocol != "" && protocol != "http/protobuf" {
		return nil, shutdown, fmt.Errorf("unsupported OTLP protocol %q: use http/protobuf", protocol)
	}

	version := "dev"
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		} else {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" && setting.Value != "" {
					version = setting.Value
					break
				}
			}
		}
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", defaultName),
			attribute.String("service.namespace", "tripgo"),
			attribute.String("service.version", version),
			attribute.String("deployment.environment", environment),
		),
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(attribute.String("service.instance.id", rand.Text())),
	)
	if err != nil {
		return nil, shutdown, fmt.Errorf("create OpenTelemetry resource: %w", err)
	}

	traceExporter, err := otlptracehttp.New(ctx, otlptracehttp.WithTimeout(2*time.Second))
	if err != nil {
		return nil, shutdown, fmt.Errorf("create OTLP trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter, sdktrace.WithBatchTimeout(time.Second)),
	)
	closers = append(closers, tp.Shutdown)

	metricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithTimeout(2*time.Second))
	if err != nil {
		return nil, shutdown, fmt.Errorf("create OTLP metric exporter: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
	)
	closers = append(closers, mp.Shutdown)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return logger, shutdown, nil
}
