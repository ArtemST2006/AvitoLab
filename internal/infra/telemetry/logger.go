// Package telemetry initializes service logging.
package telemetry

import (
	"log/slog"
	"os"
	"strings"
)

// NewLogger configures slog for the selected environment.
func NewLogger(level, env string) *slog.Logger {
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
	if env == "dev" || env == "prod" {
		return slog.New(slog.NewJSONHandler(os.Stdout, options))
	}
	return slog.New((PrettyHandlerOptions{SlogOpts: options}).NewPrettyHandler(os.Stdout))
}
