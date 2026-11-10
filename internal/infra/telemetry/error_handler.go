package telemetry

import "log/slog"

type slogErrorHandler struct {
	log *slog.Logger
}

func (h slogErrorHandler) Handle(err error) {
	h.log.Error("OpenTelemetry export failed", slog.Any("error", err))
}
