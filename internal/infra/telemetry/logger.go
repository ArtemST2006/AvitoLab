package telemetry

import (
	"io"
	"log/slog"
	"os"
	"strings"

	slogpretty "github.com/ArtemST2006/AvitoLab/internal/lib/logger/handlers"
)

// NewLogger writes JSON to stdout in every environment. LOG_FORMAT=pretty
// explicitly selects the existing PrettyHandler for local terminal runs.
func NewLogger(level, env string) *slog.Logger {
	return newLogger(level, env, os.Stdout)
}

func newLogger(level, env string, output io.Writer) *slog.Logger {
	var slogLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	options := &slog.HandlerOptions{Level: slogLevel}
	var handler slog.Handler = slog.NewJSONHandler(output, options)
	if env == "local" && strings.EqualFold(os.Getenv("LOG_FORMAT"), "pretty") {
		handler = (slogpretty.PrettyHandlerOptions{SlogOpts: options}).NewPrettyHandler(output)
	}
	handler = traceHandler{Handler: handler}
	return slog.New(handler).With(
		slog.String("service", "trip-service"),
		slog.String("environment", env),
	)
}
